package controllers

import (
	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type SettingsController struct {
	Service *services.SettingsService
}

func NewSettingsController(s *services.SettingsService) *SettingsController {
	return &SettingsController{Service: s}
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
	utils.OK(c, updated)
}
