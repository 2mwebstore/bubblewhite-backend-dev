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
	admin.Use(c.AdminRateLimiter.Middleware())
	admin.PUT("", middlewares.RequirePermission(c.DB, "settings.update"), c.Settings.Update)
	// Same LoginRateLimiter as every other "occasional, deliberate
	// action" endpoint — a full database dump is expensive enough that
	// it shouldn't be spammable at the much higher general AdminRateLimiter
	// rate just because it's an authenticated admin action.
	admin.POST("/backup/run", c.LoginRateLimiter.Middleware(), middlewares.RequirePermission(c.DB, "settings.update"), c.Backup.RunNow)
}
