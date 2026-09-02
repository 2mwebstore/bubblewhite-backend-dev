package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterSettingsRoutes(api *gin.RouterGroup, c *Container) {
	// Public — storefront reads company name, contact info, socials, logo from here.
	api.GET("/settings", c.Settings.Get)

	admin := api.Group("/admin/settings")
	admin.Use(middlewares.AuthMiddleware())
	admin.PUT("", middlewares.RequirePermission(c.DB, "settings.update"), c.Settings.Update)
}
