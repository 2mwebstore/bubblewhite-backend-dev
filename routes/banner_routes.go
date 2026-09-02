package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterBannerRoutes(api *gin.RouterGroup, c *Container) {
	// Public — storefront home page hero carousel.
	api.GET("/banners", c.Banner.ListActive)

	admin := api.Group("/admin/banners")
	admin.Use(middlewares.AuthMiddleware())
	admin.GET("", middlewares.RequirePermission(c.DB, "banner.view"), c.Banner.List)
	admin.POST("", middlewares.RequirePermission(c.DB, "banner.create"), c.Banner.Create)
	admin.PUT("/:id", middlewares.RequirePermission(c.DB, "banner.update"), c.Banner.Update)
	admin.DELETE("/:id", middlewares.RequirePermission(c.DB, "banner.delete"), c.Banner.Delete)
}
