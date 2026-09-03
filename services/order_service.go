package services

import (
	"errors"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

type OrderService struct {
	Orders         *repositories.OrderRepository
	Cart           *repositories.CartRepository
	Products       *repositories.ProductRepository
	PaymentMethods *PaymentMethodService
	Settings       *repositories.SettingsRepository
}

func NewOrderService(orders *repositories.OrderRepository, cart *repositories.CartRepository, products *repositories.ProductRepository, paymentMethods *PaymentMethodService, settings *repositories.SettingsRepository) *OrderService {
	return &OrderService{Orders: orders, Cart: cart, Products: products, PaymentMethods: paymentMethods, Settings: settings}
}

var ErrPaymentMethodDisabled = errors.New("this payment method is currently unavailable")
var ErrInvalidPaymentMethod = errors.New("invalid payment method")
var ErrAddressRequired = errors.New("delivery address is required")
var ErrPhoneRequired = errors.New("phone number is required")
var ErrPaymentNotVerified = errors.New("payment could not be verified — please try again or contact us")
var ErrOrderNotPPCBank = errors.New("order was not paid via PPCBank")
var ErrPPCBankNotConfigured = errors.New("PPCBank payment is not configured on this server")

// buildOrderItemsFromCart snapshots the customer's current cart into
// OrderItem rows (name/price/image AT THIS MOMENT) and computes the
// total — shared by Checkout (cash) and InitiatePPCBankCheckout,
// since both need the exact same cart-to-order-items logic.
func (s *OrderService) buildOrderItemsFromCart(customerID uint) ([]models.OrderItem, float64, error) {
	cartLines, err := s.Cart.FindByCustomer(customerID)
	if err != nil {
		return nil, 0, err
	}
	if len(cartLines) == 0 {
		return nil, 0, ErrEmptyCart
	}

	var total float64
	items := make([]models.OrderItem, 0, len(cartLines))
	for _, line := range cartLines {
		product, err := s.Products.FindByID(line.ProductID)
		if err != nil {
			// Product no longer exists — skip this line rather than fail
			// the whole checkout; the customer's other items still go
			// through.
			continue
		}
		// Use the SPECIFIC image the customer had previewed when they
		// added this line, same as CartService.List — falls back to the
		// product's current first image only if none was captured.
		image := line.Image
		if image == "" && len(product.Images.Data) > 0 {
			image = product.Images.Data[0]
		}
		total += product.Price * float64(line.Quantity)
		items = append(items, models.OrderItem{
			ProductID: line.ProductID,
			Name:      product.Name,
			Price:     product.Price,
			Image:     image,
			Size:      line.Size,
			Quantity:  line.Quantity,
		})
	}
	if len(items) == 0 {
		return nil, 0, ErrEmptyCart
	}
	return items, total, nil
}

// Checkout turns the customer's current cart into an Order — snapshotting
// each product's name/price/image AT THIS MOMENT into OrderItem rows, so a
// later price change or product deletion never alters what a past order
// actually showed or charged (contrast with CartService.List, which always
// shows LIVE product data). Address/phone are snapshotted the same way —
// see Order's doc comment. The cart is cleared once the order succeeds.
//
// Cash only — Bakong's integration was removed; PPCBank has its own
// separate flow (see InitiatePPCBankCheckout) since its redirect model
// doesn't fit this "verify then create" pattern.
func (s *OrderService) Checkout(customerID uint, paymentMethod, address, phone, paymentReference string) (*models.Order, error) {
	if paymentMethod != models.PaymentMethodCash {
		return nil, ErrInvalidPaymentMethod
	}
	// Admin-controlled toggle (see models.PaymentMethod) — checked
	// server-side, never trusting that the frontend only showed enabled
	// options. A customer with a stale page open, or anyone calling this
	// endpoint directly, must not be able to check out with a method the
	// business has disabled.
	enabled, err := s.PaymentMethods.IsEnabled(paymentMethod)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrPaymentMethodDisabled
	}
	if address == "" {
		return nil, ErrAddressRequired
	}
	if phone == "" {
		return nil, ErrPhoneRequired
	}

	paymentStatus := models.PaymentStatusUnpaid
	var transactionHash string

	items, subtotal, err := s.buildOrderItemsFromCart(customerID)
	if err != nil {
		return nil, err
	}
	settings, err := s.Settings.Get()
	if err != nil {
		return nil, err
	}
	total := subtotal + settings.ShippingFee

	order := &models.Order{
		CustomerID:       customerID,
		Total:            total,
		ShippingFee:      settings.ShippingFee,
		PaymentMethod:    paymentMethod,
		PaymentStatus:    paymentStatus,
		PaymentReference: paymentReference,
		TransactionHash:  transactionHash,
		Status:           models.OrderStatusPending,
		Address:          address,
		Phone:            phone,
		Items:            items,
	}
	if err := s.Orders.Create(order); err != nil {
		return nil, err
	}

	// Best-effort — the order already succeeded, so a cart-clear failure
	// shouldn't fail the checkout response; it would just leave stale
	// items behind the customer can remove manually.
	_ = s.Cart.DeleteAllForCustomer(customerID)

	return order, nil
}

// InitiatePPCBankCheckout creates a PENDING, UNPAID order immediately —
// unlike Checkout() for cash, which creates the order in one step with no
// separate payment confirmation to wait for. This difference is
// necessary, not a preference: PPCBank's Generate KHQR Payment API returns
// a redirect URL (not a QR string we render ourselves), so the customer
// physically leaves this site to pay on PPCBank's hosted page. There has
// to be a real order in the database — identified by its own Reference(), e.g.
// "BW-000042" — for PPCBank's success redirect and webhook to reconcile
// against when the customer returns or the async notification arrives.
func (s *OrderService) InitiatePPCBankCheckout(customerID uint, address, phone string) (*models.Order, string, error) {
	enabled, err := s.PaymentMethods.IsEnabled(models.PaymentMethodPPCBank)
	if err != nil {
		return nil, "", err
	}
	if !enabled {
		return nil, "", ErrPaymentMethodDisabled
	}
	if address == "" {
		return nil, "", ErrAddressRequired
	}
	if phone == "" {
		return nil, "", ErrPhoneRequired
	}

	items, subtotal, err := s.buildOrderItemsFromCart(customerID)
	if err != nil {
		return nil, "", err
	}
	settings, err := s.Settings.Get()
	if err != nil {
		return nil, "", err
	}
	total := subtotal + settings.ShippingFee

	order := &models.Order{
		CustomerID:    customerID,
		Total:         total,
		ShippingFee:   settings.ShippingFee,
		PaymentMethod: models.PaymentMethodPPCBank,
		PaymentStatus: models.PaymentStatusUnpaid,
		Status:        models.OrderStatusPending,
		Address:       address,
		Phone:         phone,
		Items:         items,
	}
	if err := s.Orders.Create(order); err != nil {
		return nil, "", err
	}

	// The order's own Reference() (e.g. "BW-000042") becomes PPCBank's
	// billNumber — what ties the redirect/webhook back to this specific
	// order later. Only known once the order actually has an ID, hence
	// this second save.
	billNumber := order.Reference()
	order.PaymentReference = billNumber
	order.Items = nil // same reasoning as UpdateStatus
	if err := s.Orders.Update(order); err != nil {
		return order, "", err
	}

	paymentURL, err := GeneratePPCBankKHQRPayment(billNumber, total, "USD")
	if err != nil {
		// The order already exists as pending/unpaid — deliberately NOT
		// deleted on this failure. It'll just sit there until the
		// customer retries or an admin follows up; better than silently
		// losing the customer's order intent, and the cart is still
		// intact below since we haven't cleared it yet.
		if errors.Is(err, ErrPPCBankNotSet) {
			return order, "", ErrPPCBankNotConfigured
		}
		return order, "", err
	}

	// Only cleared once we know the customer actually has somewhere to
	// go pay — if GeneratePPCBankKHQRPayment failed above, their cart
	// stays intact for a clean retry.
	_ = s.Cart.DeleteAllForCustomer(customerID)

	return order, paymentURL, nil
}

// VerifyPPCBankPayment re-checks an order's payment status against
// PPCBank's real Check KHQR Payment Status API. Used by BOTH the
// webhook handler (which never trusts the webhook payload's
// paymentStatus field alone — PPCBank's docs describe no signature/auth
// mechanism on that webhook) and the admin's manual "re-check payment"
// action.
func (s *OrderService) VerifyPPCBankPayment(orderID uint) (*models.Order, error) {
	order, err := s.Orders.FindByIDAdmin(orderID)
	if err != nil {
		return nil, err
	}
	if order.PaymentMethod != models.PaymentMethodPPCBank || order.PaymentReference == "" {
		return nil, ErrOrderNotPPCBank
	}

	paid, detail, err := CheckPPCBankPaymentStatus(order.PaymentReference)
	if err != nil {
		return nil, err
	}

	// Captured before
	// mutating, so this only fires on the actual unpaid→paid transition,
	// not every time this webhook/manual-recheck runs against an order
	// that's already paid.
	wasAlreadyPaid := order.PaymentStatus == models.PaymentStatusPaid

	if paid {
		order.PaymentStatus = models.PaymentStatusPaid
		if detail != nil && order.TransactionHash == "" {
			order.TransactionHash = detail.TransactionHash
		}
		if detail != nil && order.Invoice == "" {
			order.Invoice = string(detail.ReferenceNo)
		}
	}
	order.Items = nil // same reasoning as UpdateStatus above
	if err := s.Orders.Update(order); err != nil {
		return nil, err
	}
	if paid && !wasAlreadyPaid {
		NotifyOrderPaid(order)
	}
	return s.Orders.FindByIDAdmin(orderID)
}

// ListByCustomer returns a page of this customer's orders, newest first.
// Used both by the customer's own order history (client-facing pagination
// + infinite scroll) and the admin's "view this customer's orders" panel.
func (s *OrderService) ListByCustomer(customerID uint, page utils.PageParams) ([]models.Order, int64, error) {
	customerScope := func(db *gorm.DB) *gorm.DB { return db.Where("customer_id = ?", customerID) }
	total, err := s.Orders.Count(customerScope)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.Orders.FindByCustomer(customerID, page.Scope())
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *OrderService) GetByIDForCustomer(id, customerID uint) (*models.Order, error) {
	return s.Orders.FindByIDForCustomer(id, customerID)
}

// --- Admin ---

// ListAllAdmin lists orders newest-first, filtered by the given
// OrderFilter (any empty field means "no filter on that dimension"). The
// same filter scope is applied to both the Count and the actual page
// fetch, so pagination totals stay accurate when filtered — counting
// unfiltered rows while returning filtered ones would show a wrong
// "page 1 of N" for any filtered view.
func (s *OrderService) ListAllAdmin(page utils.PageParams, filter repositories.OrderFilter) ([]models.Order, int64, error) {
	scope := filter.Scope()

	total, err := s.Orders.Count(scope)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.Orders.FindAllAdmin(scope, page.Scope())
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *OrderService) GetByIDAdmin(id uint) (*models.Order, error) {
	return s.Orders.FindByIDAdmin(id)
}

// UpdateStatus lets an admin move an order through its lifecycle
// (pending -> confirmed -> shipped -> completed, or cancelled).
func (s *OrderService) UpdateStatus(id uint, status string) (*models.Order, error) {
	order, err := s.Orders.FindByIDAdmin(id)
	if err != nil {
		return nil, err
	}
	order.Status = status
	// Clear the preloaded Items association before saving — GORM's Save()
	// would otherwise try to re-upsert every item too, which is both
	// unnecessary and risky (a struct that came from Preload isn't meant
	// to be treated as "these are the items to write").
	order.Items = nil
	if err := s.Orders.Update(order); err != nil {
		return nil, err
	}
	// Re-fetch with items populated for the response.
	return s.Orders.FindByIDAdmin(id)
}
