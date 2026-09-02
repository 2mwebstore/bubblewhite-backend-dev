package controllers

import (
	"errors"
	"log"
	"strconv"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type AdminOrderController struct {
	Service *services.OrderService
}

func NewAdminOrderController(s *services.OrderService) *AdminOrderController {
	return &AdminOrderController{Service: s}
}

// GET /api/admin/orders (requires order.view)
// Optional filters: ?search=, ?customerId=, ?status=, ?paymentMethod=
func (ctrl *AdminOrderController) List(c *gin.Context) {
	page := utils.ParsePageParams(c)
	filter := repositories.OrderFilter{
		Search:        c.Query("search"),
		CustomerID:    c.Query("customerId"),
		Status:        c.Query("status"),
		PaymentMethod: c.Query("paymentMethod"),
	}
	orders, total, err := ctrl.Service.ListAllAdmin(page, filter)
	if err != nil {
		utils.InternalError(c, "failed to fetch orders")
		return
	}
	utils.OKWithMeta(c, orders, page.BuildMeta(total))
}

// GET /api/admin/customers/:id/orders (requires customer.view) — a
// specific customer's full order history, for the admin customer detail
// page. Reuses OrderService.ListByCustomer — the same method the
// customer's OWN order history endpoint uses, since there's no ownership
// restriction to differ here (this route itself is what enforces "only an
// admin can view an arbitrary customer's orders").
func (ctrl *AdminOrderController) ListByCustomer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid customer id")
		return
	}
	orders, err := ctrl.Service.ListByCustomer(uint(id))
	if err != nil {
		utils.InternalError(c, "failed to fetch orders")
		return
	}
	utils.OK(c, orders)
}

// GET /api/admin/orders/:id (requires order.view)
func (ctrl *AdminOrderController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid order id")
		return
	}
	order, err := ctrl.Service.GetByIDAdmin(uint(id))
	if err != nil {
		utils.NotFound(c, "order not found")
		return
	}
	utils.OK(c, order)
}

type updateOrderStatusInput struct {
	Status string `json:"status" validate:"required,oneof=pending confirmed shipped completed cancelled"`
}

// PATCH /api/admin/orders/:id/status (requires order.manage)
func (ctrl *AdminOrderController) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid order id")
		return
	}

	var in updateOrderStatusInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	order, err := ctrl.Service.UpdateStatus(uint(id), in.Status)
	if err != nil {
		utils.InternalError(c, "failed to update order status")
		return
	}
	utils.OK(c, order)
}

// POST /api/admin/orders/:id/verify-payment (requires order.manage)
// Re-checks a Bakong order's payment status against Bakong's real API —
// for when a customer says they paid but the order still shows unpaid.
func (ctrl *AdminOrderController) VerifyPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid order id")
		return
	}

	// Fetch first to know which payment method this order actually used
	// — the same "re-check payment" button on the admin order detail
	// page now works for both Bakong and PPCBank orders, dispatching to
	// whichever real API actually backs that order.
	existing, err := ctrl.Service.GetByIDAdmin(uint(id))
	if err != nil {
		utils.NotFound(c, "order not found")
		return
	}

	var order *models.Order
	switch existing.PaymentMethod {
	case models.PaymentMethodBakong:
		order, err = ctrl.Service.VerifyBakongPayment(uint(id))
	case models.PaymentMethodPPCBank:
		order, err = ctrl.Service.VerifyPPCBankPayment(uint(id))
	default:
		utils.BadRequest(c, "this order was not paid via Bakong or PPCBank")
		return
	}
	if err != nil {
		if errors.Is(err, services.ErrOrderNotBakong) {
			utils.BadRequest(c, "this order was not paid via Bakong")
			return
		}
		if errors.Is(err, services.ErrOrderNotPPCBank) {
			utils.BadRequest(c, "this order was not paid via PPCBank")
			return
		}
		log.Printf("admin verify payment: failed for order %d: %v", id, err)
		utils.InternalError(c, "failed to verify payment")
		return
	}
	utils.OK(c, order)
}

// GET /api/admin/orders/:id/transaction-detail (requires order.view)
// Full Bakong transaction detail — tracking status, receiver bank, sender
// account — for an admin investigating a specific payment. Read-only, so
// gated on order.view (same as viewing the order itself) rather than
// order.manage — this doesn't change anything, unlike VerifyPayment above.
func (ctrl *AdminOrderController) GetTransactionDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid order id")
		return
	}

	detail, err := ctrl.Service.GetTransactionDetail(uint(id))
	if err != nil {
		if errors.Is(err, services.ErrOrderNotBakong) {
			utils.BadRequest(c, "this order was not paid via Bakong")
			return
		}
		utils.NotFound(c, err.Error())
		return
	}
	utils.OK(c, detail)
}
