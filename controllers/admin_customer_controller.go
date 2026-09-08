package controllers

import (
	"strconv"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

// AdminCustomerController is deliberately separate from CustomerController
// (customer self-service) — these endpoints are gated by admin
// AuthMiddleware + RequirePermission, never CustomerAuthMiddleware, so a
// customer token can never reach them.
type AdminCustomerController struct {
	Service *services.CustomerService
	Audit   *services.AuditLogService
}

func NewAdminCustomerController(s *services.CustomerService, audit *services.AuditLogService) *AdminCustomerController {
	return &AdminCustomerController{Service: s, Audit: audit}
}

// GET /api/admin/customers (requires customer.view)
func (ctrl *AdminCustomerController) List(c *gin.Context) {
	page := utils.ParsePageParams(c)
	customers, total, err := ctrl.Service.ListAllAdmin(page)
	if err != nil {
		utils.InternalError(c, "failed to fetch customers")
		return
	}
	out := make([]gin.H, 0, len(customers))
	for i := range customers {
		out = append(out, customerJSON(&customers[i]))
	}
	utils.OKWithMeta(c, out, page.BuildMeta(total))
}

// GET /api/admin/customers/:id (requires customer.view) — single customer
// detail, for the admin customer detail page (linked from the order list's
// customer column).
func (ctrl *AdminCustomerController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid customer id")
		return
	}
	customer, err := ctrl.Service.GetByID(uint(id))
	if err != nil {
		utils.NotFound(c, "customer not found")
		return
	}
	utils.OK(c, customerJSON(customer))
}

type adminResetCustomerPasswordInput struct {
	NewPassword string `json:"newPassword" validate:"required,min=6"`
}

// PATCH /api/admin/customers/:id/password (requires customer.manage)
// Admin-initiated reset — no current password needed, same pattern as
// resetting a staff user's password.
func (ctrl *AdminCustomerController) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid customer id")
		return
	}

	var in adminResetCustomerPasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	if err := ctrl.Service.AdminResetPassword(uint(id), in.NewPassword); err != nil {
		utils.InternalError(c, "failed to reset password")
		return
	}
	ip, ua := auditContext(c)
	label := c.Param("id")
	if customer, err := ctrl.Service.GetByID(uint(id)); err == nil {
		label = customer.Name
	}
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "reset_password", Resource: "customer", ResourceID: c.Param("id"), ResourceLabel: label,
		Description: "Reset a customer's password", IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"message": "password reset"})
}

type setActiveInput struct {
	IsActive *bool `json:"isActive" validate:"required"`
}

// PATCH /api/admin/customers/:id/active (requires customer.manage)
// Enables/disables a customer's ability to log in, without deleting their
// account or order history.
func (ctrl *AdminCustomerController) SetActive(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid customer id")
		return
	}

	var in setActiveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if in.IsActive == nil {
		utils.BadRequest(c, "isActive is required")
		return
	}

	if err := ctrl.Service.SetActive(uint(id), *in.IsActive); err != nil {
		utils.InternalError(c, "failed to update customer")
		return
	}
	ip, ua := auditContext(c)
	label := c.Param("id")
	if customer, err := ctrl.Service.GetByID(uint(id)); err == nil {
		label = customer.Name
	}
	action := "deactivate"
	if *in.IsActive {
		action = "activate"
	}
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: action, Resource: "customer", ResourceID: c.Param("id"), ResourceLabel: label,
		Description: "Changed customer account status", IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"message": "updated"})
}
