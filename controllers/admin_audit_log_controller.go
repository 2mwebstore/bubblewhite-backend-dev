package controllers

import (
	"errors"
	"fmt"

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

// DELETE /api/admin/audit-logs/staff?keep=<preset> and
// /customers?keep=<preset> (requires audit.view) — the "keep only the
// last N months" cleanup buttons. preset must be one of "this_month",
// "last_month", "3_months", "5_months" — see
// AuditLogService.retentionCutoff for exactly what cutoff date each one
// computes. Logs the cleanup itself as a new audit entry once it
// completes — that new entry is created AFTER the delete runs, so it's
// never at risk of deleting itself, and gives a real accountability
// trail for who cleared out old log entries and when.
func (ctrl *AdminAuditLogController) CleanupStaff(c *gin.Context) {
	ctrl.cleanup(c, "admin")
}

func (ctrl *AdminAuditLogController) CleanupCustomers(c *gin.Context) {
	ctrl.cleanup(c, "customer")
}

func (ctrl *AdminAuditLogController) cleanup(c *gin.Context, actorType string) {
	preset := c.Query("keep")
	deleted, err := ctrl.Service.Cleanup(actorType, preset)
	if err != nil {
		if errors.Is(err, services.ErrInvalidRetentionPreset) {
			utils.BadRequest(c, "invalid retention preset")
			return
		}
		utils.InternalError(c, "failed to clean up audit logs")
		return
	}

	ip, ua := auditContext(c)
	ctrl.Service.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "cleanup", Resource: "audit_log",
		Description: fmt.Sprintf("Deleted %d %s audit log entries (kept: %s)", deleted, actorType, preset),
		IPAddress:   ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"deleted": deleted})
}

type deleteSelectedInput struct {
	IDs []uint `json:"ids" validate:"required,min=1"`
}

// POST /api/admin/audit-logs/staff/delete-selected and
// /customers/delete-selected (requires audit.manage) — the checkbox-based
// "select these specific rows, delete them" flow, distinct from the
// preset-based Cleanup endpoint above. A dedicated POST endpoint rather
// than DELETE-with-a-body: some proxies/middleware handle a body on a
// DELETE request inconsistently, so a POST with a clear, unambiguous
// action name in the URL avoids relying on that working correctly.
func (ctrl *AdminAuditLogController) DeleteSelectedStaff(c *gin.Context) {
	ctrl.deleteSelected(c, "admin")
}

func (ctrl *AdminAuditLogController) DeleteSelectedCustomers(c *gin.Context) {
	ctrl.deleteSelected(c, "customer")
}

func (ctrl *AdminAuditLogController) deleteSelected(c *gin.Context, actorType string) {
	var in deleteSelectedInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	deleted, err := ctrl.Service.DeleteSelected(actorType, in.IDs)
	if err != nil {
		utils.InternalError(c, "failed to delete selected audit logs")
		return
	}

	ip, ua := auditContext(c)
	ctrl.Service.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "delete_selected", Resource: "audit_log",
		Description: fmt.Sprintf("Deleted %d selected %s audit log entries", deleted, actorType),
		IPAddress:   ip, UserAgent: ua,
	})
	utils.OK(c, gin.H{"deleted": deleted})
}
