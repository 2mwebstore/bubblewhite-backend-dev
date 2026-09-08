package controllers

import (
	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type AdminAuditLogController struct {
	Service *services.AuditLogService
}

func NewAdminAuditLogController(s *services.AuditLogService) *AdminAuditLogController {
	return &AdminAuditLogController{Service: s}
}

// auditContext pulls the IP/user-agent every audit log entry records —
// shared here so every controller that calls AuditLogService.Log fills
// these in the same, correct way (RealClientIP specifically, not Gin's
// raw ClientIP() — see that function's own doc comment for why, given
// this app runs behind Railway's edge proxy) rather than each call site
// reimplementing it slightly differently.
func auditContext(c *gin.Context) (ip, userAgent string) {
	return middlewares.RealClientIP(c), c.Request.UserAgent()
}

// GET /api/admin/audit-logs/staff (requires audit.view) — staff/admin
// activity only. ActorType is hardcoded here, not read from the query
// string, so this route can never be tricked into returning customer
// entries — that separation is what makes these genuinely "two views"
// rather than one filterable list a caller could bypass.
func (ctrl *AdminAuditLogController) ListStaff(c *gin.Context) {
	ctrl.list(c, "admin")
}

// GET /api/admin/audit-logs/customers (requires audit.view) — customer
// activity only. Same ActorID-optional support as ListStaff — passing
// ?actorId= scopes this to one specific customer's history, which is how
// this same endpoint also powers a "recent activity" section embedded in
// an individual customer's own admin detail page, not just the
// standalone full log view.
func (ctrl *AdminAuditLogController) ListCustomers(c *gin.Context) {
	ctrl.list(c, "customer")
}

func (ctrl *AdminAuditLogController) list(c *gin.Context, actorType string) {
	page := utils.ParsePageParams(c)
	filter := repositories.AuditLogFilter{
		ActorType: actorType,
		ActorID:   c.Query("actorId"),
		Action:    c.Query("action"),
		Resource:  c.Query("resource"),
		Search:    c.Query("search"),
		DateFrom:  c.Query("dateFrom"),
		DateTo:    c.Query("dateTo"),
	}
	logs, total, err := ctrl.Service.List(page, filter)
	if err != nil {
		utils.InternalError(c, "failed to fetch audit logs")
		return
	}
	utils.OKWithMeta(c, logs, page.BuildMeta(total))
}

// GET /api/admin/audit-logs/staff/filters and /customers/filters
// (requires audit.view) — the distinct action/resource values available
// for that actor type right now, for populating the filter dropdowns.
func (ctrl *AdminAuditLogController) StaffFilterOptions(c *gin.Context) {
	ctrl.filterOptions(c, "admin")
}

func (ctrl *AdminAuditLogController) CustomerFilterOptions(c *gin.Context) {
	ctrl.filterOptions(c, "customer")
}

func (ctrl *AdminAuditLogController) filterOptions(c *gin.Context, actorType string) {
	options, err := ctrl.Service.FilterOptions(actorType)
	if err != nil {
		utils.InternalError(c, "failed to fetch filter options")
		return
	}
	utils.OK(c, options)
}
