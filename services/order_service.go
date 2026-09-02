package services

import (
	"errors"

	"bubblewhite-backend/config"
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

type OrderService struct {
	Orders         *repositories.OrderRepository
	Cart           *repositories.CartRepository
	Products       *repositories.ProductRepository
	PaymentMethods *PaymentMethodService
}

func NewOrderService(orders *repositories.OrderRepository, cart *repositories.CartRepository, products *repositories.ProductRepository, paymentMethods *PaymentMethodService) *OrderService {
	return &OrderService{Orders: orders, Cart: cart, Products: products, PaymentMethods: paymentMethods}
}

var ErrPaymentMethodDisabled = errors.New("this payment method is currently unavailable")
var ErrInvalidPaymentMethod = errors.New("invalid payment method")
var ErrAddressRequired = errors.New("delivery address is required")
var ErrPhoneRequired = errors.New("phone number is required")
var ErrPaymentReferenceRequired = errors.New("payment reference is required for Bakong payments")
var ErrPaymentNotVerified = errors.New("payment could not be verified — please try again or contact us")
var ErrBakongNotConfigured = errors.New("Bakong payment verification is not configured on this server")
var ErrOrderNotBakong = errors.New("order was not paid via Bakong")
var ErrOrderNotPPCBank = errors.New("order was not paid via PPCBank")
var ErrPPCBankNotConfigured = errors.New("PPCBank payment is not configured on this server")

// buildOrderItemsFromCart snapshots the customer's current cart into
// OrderItem rows (name/price/image AT THIS MOMENT) and computes the
// total — shared by Checkout (cash/Bakong) and InitiatePPCBankCheckout,
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
// For Bakong payments, this INDEPENDENTLY verifies the payment against
// Bakong's real API before creating the order — the frontend already gates
// this in its own UI (BakongPaymentModal never lets a customer submit
// without a confirmed payment), but that's a UI convenience, not a
// security boundary. Anyone could call this endpoint directly with a
// forged "I paid" claim; the only thing that actually matters is what
// Bakong itself reports.
func (s *OrderService) Checkout(customerID uint, paymentMethod, address, phone, paymentReference string) (*models.Order, error) {
	if paymentMethod != models.PaymentMethodCash && paymentMethod != models.PaymentMethodBakong {
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
	if paymentMethod == models.PaymentMethodBakong {
		if paymentReference == "" {
			return nil, ErrPaymentReferenceRequired
		}
		paid, detail, err := CheckBakongPaymentByMD5(paymentReference)
		if err != nil {
			// Distinct from "checked with Bakong and it's genuinely not
			// paid" — this specific case means the GO backend's OWN
			// BAKONG_API_EMAIL isn't set (a separate deployment/env from
			// wherever the Nuxt frontend's polling runs, so the frontend
			// can report a successful check while this independent
			// server-side check fails for an entirely different reason).
			// Surfaced as a distinct error so this is diagnosable from the
			// API response itself instead of looking identical to a
			// genuine unpaid order.
			if errors.Is(err, ErrBakongNotSet) {
				return nil, ErrBakongNotConfigured
			}
			return nil, ErrPaymentNotVerified
		}
		if !paid {
			return nil, ErrPaymentNotVerified
		}
		paymentStatus = models.PaymentStatusPaid
		if detail != nil {
			transactionHash = detail.Hash
		}
	}

	items, total, err := s.buildOrderItemsFromCart(customerID)
	if err != nil {
		return nil, err
	}

	order := &models.Order{
		CustomerID:       customerID,
		Total:            total,
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

	// Only for orders that are ACTUALLY paid at creation time (Bakong,
	// already independently verified above) — a cash order is still
	// unpaid at this point (paid on delivery), so it correctly gets no
	// notification here; nothing else in this function later flips a
	// cash order to paid, so there's no other spot in Checkout() that
	// would need this same call.
	if order.PaymentStatus == models.PaymentStatusPaid {
		NotifyOrderPaid(order)
	}

	// Best-effort — the order already succeeded, so a cart-clear failure
	// shouldn't fail the checkout response; it would just leave stale
	// items behind the customer can remove manually.
	_ = s.Cart.DeleteAllForCustomer(customerID)

	return order, nil
}

// InitiatePPCBankCheckout creates a PENDING, UNPAID order immediately —
// unlike Checkout() for Bakong, which only creates the order AFTER
// payment is already confirmed. This difference is necessary, not a
// preference: PPCBank's Generate KHQR Payment API returns a redirect URL
// (not a QR string we render ourselves), so the customer physically
// leaves this site to pay on PPCBank's hosted page. There has to be a
// real order in the database — identified by its own Reference(), e.g.
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

	items, total, err := s.buildOrderItemsFromCart(customerID)
	if err != nil {
		return nil, "", err
	}

	order := &models.Order{
		CustomerID:    customerID,
		Total:         total,
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
	order.Items = nil // same reasoning as UpdateStatus/VerifyBakongPayment
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
// action, same pattern as VerifyBakongPayment.
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

	// Same reasoning as VerifyBakongPayment above — captured before
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

func (s *OrderService) ListByCustomer(customerID uint) ([]models.Order, error) {
	return s.Orders.FindByCustomer(customerID)
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

// VerifyBakongPayment re-checks an order's payment status against Bakong's
// real API — the admin's manual "re-check payment" action, e.g. when a
// customer says they paid but the order still shows unpaid (a poll that
// missed the exact moment during checkout, or the customer paid slightly
// after their session's polling window closed).
func (s *OrderService) VerifyBakongPayment(orderID uint) (*models.Order, error) {
	order, err := s.Orders.FindByIDAdmin(orderID)
	if err != nil {
		return nil, err
	}
	if order.PaymentMethod != models.PaymentMethodBakong || order.PaymentReference == "" {
		return nil, ErrOrderNotBakong
	}

	paid, detail, err := CheckBakongPaymentByMD5(order.PaymentReference)
	if err != nil {
		return nil, err
	}

	// Captured before mutating — only notify on the actual unpaid→paid
	// TRANSITION, not every time this gets called again on an order
	// that's already paid (e.g. an admin clicking "re-verify" a second
	// time), which would otherwise spam the Telegram group with
	// duplicate alerts for the same order.
	wasAlreadyPaid := order.PaymentStatus == models.PaymentStatusPaid

	if paid {
		order.PaymentStatus = models.PaymentStatusPaid
		if detail != nil && order.TransactionHash == "" {
			order.TransactionHash = detail.Hash
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

// GetTransactionDetail fetches Bakong's FULL transaction detail for an
// order — tracking status, receiver bank, receiver bank account, sender
// account — using check_transaction_by_hash. This is richer than the
// pass/fail check used for checkout/VerifyBakongPayment, for the admin
// who needs to actually investigate a specific payment (e.g. confirming
// which bank account the money landed in).
// TransactionDetailView wraps BakongTransactionDetail with an explicit,
// server-computed match check — AccountMatch tells the admin definitively
// whether this payment actually landed in the business's own configured
// account, rather than making them manually eyeball toAccountId against
// BAKONG_ACCOUNT_ID themselves. ExpectedAccountID is included so the
// mismatch (if any) is visible without needing a separate lookup.
type TransactionDetailView struct {
	*BakongTransactionDetail
	AccountMatch      bool   `json:"accountMatch"`
	ExpectedAccountID string `json:"expectedAccountId"`
}

func (s *OrderService) GetTransactionDetail(orderID uint) (*TransactionDetailView, error) {
	order, err := s.Orders.FindByIDAdmin(orderID)
	if err != nil {
		return nil, err
	}
	if order.PaymentMethod != models.PaymentMethodBakong {
		return nil, ErrOrderNotBakong
	}
	if order.TransactionHash == "" {
		return nil, errors.New("no Bakong transaction hash recorded for this order yet")
	}

	detail, err := GetBakongTransactionByHash(order.TransactionHash)
	if err != nil {
		return nil, err
	}

	expected := config.Get().BakongAccountID
	return &TransactionDetailView{
		BakongTransactionDetail: detail,
		// Only claim a match if BOTH sides are actually non-empty — an
		// unconfigured BAKONG_ACCOUNT_ID must never accidentally read as
		// "matches," which an empty-string comparison could otherwise do.
		AccountMatch:      expected != "" && detail.ToAccountID == expected,
		ExpectedAccountID: expected,
	}, nil
}
