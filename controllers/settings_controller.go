package controllers

import (
	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type SettingsController struct {
	Service *services.SettingsService
	Audit   *services.AuditLogService
}

func NewSettingsController(s *services.SettingsService, audit *services.AuditLogService) *SettingsController {
	return &SettingsController{Service: s, Audit: audit}
}

// GET /api/settings — public, the storefront reads company/contact info from here.
func (ctrl *SettingsController) Get(c *gin.Context) {
	settings, err := ctrl.Service.Get()
	if err != nil {
		utils.InternalError(c, "failed to fetch settings")
		return
	}
	utils.OK(c, settings)
}

// PUT /api/admin/settings (requires settings.update)
// Admin panel edits company name/detail and contact info from here.
func (ctrl *SettingsController) Update(c *gin.Context) {
	var patch models.Settings
	if err := c.ShouldBindJSON(&patch); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	updated, err := ctrl.Service.Update(&patch)
	if err != nil {
		utils.InternalError(c, "failed to update settings")
		return
	}
	ip, ua := auditContext(c)
	ctrl.Audit.Log(services.LogEntry{
		ActorType: "admin", ActorID: middlewares.CurrentUserID(c), ActorName: middlewares.CurrentUserEmail(c),
		Action: "update", Resource: "settings", Description: "Updated site settings",
		IPAddress: ip, UserAgent: ua,
	})
	utils.OK(c, updated)
}
