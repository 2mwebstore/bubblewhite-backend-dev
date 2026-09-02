package controllers

import (
	"errors"
	"log"
	"strconv"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type OrderController struct {
	Service *services.OrderService
}

func NewOrderController(s *services.OrderService) *OrderController {
	return &OrderController{Service: s}
}

type checkoutInput struct {
	PaymentMethod string `json:"paymentMethod" validate:"required,oneof=cash bakong"`
	Address       string `json:"address" validate:"required"`
	Phone         string `json:"phone" validate:"required"`
	// PaymentReference is the MD5 hash bakong-khqr generated for this
	// checkout's KHQR (see BakongPaymentModal.vue) — required for Bakong,
	// ignored for cash. This is only a POINTER to what to check; the
	// actual payment confirmation always comes from Bakong's own API
	// inside Checkout(), never from trusting this request.
	PaymentReference string `json:"paymentReference"`
}

// POST /api/customer/orders — requires customer auth — checkout the
// current cart into a new order.
func (ctrl *OrderController) Checkout(c *gin.Context) {
	var in checkoutInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	order, err := ctrl.Service.Checkout(middlewares.CurrentCustomerID(c), in.PaymentMethod, in.Address, in.Phone, in.PaymentReference)
	if err != nil {
		if errors.Is(err, services.ErrEmptyCart) {
			utils.BadRequest(c, "រទេះទំនិញរបស់អ្នកទទេ")
			return
		}
		if errors.Is(err, services.ErrInvalidPaymentMethod) {
			utils.BadRequest(c, "invalid payment method")
			return
		}
		if errors.Is(err, services.ErrPaymentMethodDisabled) {
			utils.BadRequest(c, "វិធីទូទាត់នេះមិនអាចប្រើប្រាស់បានទេនាពេលនេះ")
			return
		}
		if errors.Is(err, services.ErrAddressRequired) {
			utils.FailWithErrors(c, map[string]string{"address": "សូមបញ្ចូលអាសយដ្ឋានដឹកជញ្ជូន"})
			return
		}
		if errors.Is(err, services.ErrPhoneRequired) {
			utils.FailWithErrors(c, map[string]string{"phone": "សូមបញ្ចូលលេខទូរស័ព្ទ"})
			return
		}
		if errors.Is(err, services.ErrPaymentReferenceRequired) {
			utils.BadRequest(c, "payment reference is required for Bakong payments")
			return
		}
		if errors.Is(err, services.ErrBakongNotConfigured) {
			// A different, more actionable message than the generic
			// "try again" — retrying won't help here, the server itself
			// needs BAKONG_API_EMAIL configured before Bakong checkout can
			// ever succeed.
			utils.BadRequest(c, "ការទូទាត់ Bakong មិនទាន់អាចប្រើប្រាស់បានទេ សូមទាក់ទងអ្នកគ្រប់គ្រង")
			return
		}
		if errors.Is(err, services.ErrPaymentNotVerified) {
			utils.BadRequest(c, "មិនអាចផ្ទៀងផ្ទាត់ការទូទាត់បានទេ សូមព្យាយាមម្តងទៀត")
			return
		}
		utils.InternalError(c, "failed to place order")
		return
	}
	utils.Created(c, order)
}

type initiatePPCBankInput struct {
	Address string `json:"address" validate:"required"`
	Phone   string `json:"phone" validate:"required"`
}

// POST /api/customer/orders/ppcbank/initiate — requires customer auth.
// Creates a PENDING order immediately and returns a PPCBank paymentURL to
// redirect the customer to — see OrderService.InitiatePPCBankCheckout for
// why this differs from the cash/Bakong Checkout() flow above.
func (ctrl *OrderController) InitiatePPCBankCheckout(c *gin.Context) {
	var in initiatePPCBankInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	order, paymentURL, err := ctrl.Service.InitiatePPCBankCheckout(middlewares.CurrentCustomerID(c), in.Address, in.Phone)
	if err != nil {
		if errors.Is(err, services.ErrEmptyCart) {
			utils.BadRequest(c, "រទេះទំនិញរបស់អ្នកទទេ")
			return
		}
		if errors.Is(err, services.ErrPaymentMethodDisabled) {
			utils.BadRequest(c, "វិធីទូទាត់នេះមិនអាចប្រើប្រាស់បានទេនាពេលនេះ")
			return
		}
		if errors.Is(err, services.ErrAddressRequired) {
			utils.FailWithErrors(c, map[string]string{"address": "សូមបញ្ចូលអាសយដ្ឋានដឹកជញ្ជូន"})
			return
		}
		if errors.Is(err, services.ErrPhoneRequired) {
			utils.FailWithErrors(c, map[string]string{"phone": "សូមបញ្ចូលលេខទូរស័ព្ទ"})
			return
		}
		if errors.Is(err, services.ErrPPCBankNotConfigured) {
			utils.BadRequest(c, "ការទូទាត់ PPCBank មិនទាន់អាចប្រើប្រាស់បានទេ សូមទាក់ទងអ្នកគ្រប់គ្រង")
			return
		}
		// Logged before falling through — without this, whatever ACTUALLY
		// went wrong (wrong credentials, IP not yet whitelisted with
		// PPCBank, unreachable API, a real bug) is invisible to both the
		// customer and whoever's debugging this, since none of the
		// specific error checks above matched.
		log.Printf("ppcbank checkout: InitiatePPCBankCheckout failed for customer %d: %v", middlewares.CurrentCustomerID(c), err)
		utils.InternalError(c, "failed to initiate PPCBank checkout")
		return
	}
	utils.OK(c, gin.H{"orderId": order.ID, "reference": order.Reference(), "paymentURL": paymentURL})
}

// GET /api/customer/orders/ppcbank/status?billNumber=BW-000042 — requires
// customer auth. Used by the /orders/ppcbank-return page the customer
// lands on after PPCBank redirects them back. This is the ideal moment
// to actively re-verify with PPCBank directly — the customer is HERE
// because they just finished (or abandoned) paying, so there's no reason
// to passively wait on the webhook, which per PPCBank's own docs has no
// retry if it fails to reach us.
func (ctrl *OrderController) PPCBankReturnStatus(c *gin.Context) {
	billNumber := c.Query("billNumber")
	if billNumber == "" {
		utils.BadRequest(c, "billNumber is required")
		return
	}
	orderID, err := models.ParseOrderReference(billNumber)
	if err != nil {
		utils.BadRequest(c, "invalid order reference")
		return
	}

	// Ownership check FIRST — never let one customer probe another's
	// order status just by guessing/incrementing a billNumber.
	order, err := ctrl.Service.GetByIDForCustomer(orderID, middlewares.CurrentCustomerID(c))
	if err != nil {
		utils.NotFound(c, "order not found")
		return
	}

	if order.PaymentMethod == models.PaymentMethodPPCBank && order.PaymentStatus != models.PaymentStatusPaid {
		if verified, err := ctrl.Service.VerifyPPCBankPayment(orderID); err == nil {
			order = verified
		} else {
			// Logged — without this, "genuinely not paid yet" and "the
			// verification call itself is broken" look completely
			// identical from the customer's side (both just show "not
			// yet paid"), with nothing anywhere to tell them apart.
			log.Printf("ppcbank return status: VerifyPPCBankPayment failed for order %d (billNumber %s): %v", orderID, billNumber, err)
		}
		// Still falls through to return the order's last-known status
		// rather than erroring the whole page — the webhook or a later
		// admin re-check can still resolve it even if this specific
		// verification attempt failed.
	}

	utils.OK(c, order)
}

// GET /api/customer/orders — requires customer auth — order history list
func (ctrl *OrderController) List(c *gin.Context) {
	orders, err := ctrl.Service.ListByCustomer(middlewares.CurrentCustomerID(c))
	if err != nil {
		utils.InternalError(c, "failed to fetch orders")
		return
	}
	utils.OK(c, orders)
}

// GET /api/customer/orders/:id — requires customer auth — order detail
func (ctrl *OrderController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid order id")
		return
	}
	order, err := ctrl.Service.GetByIDForCustomer(uint(id), middlewares.CurrentCustomerID(c))
	if err != nil {
		utils.NotFound(c, "order not found")
		return
	}
	utils.OK(c, order)
}
