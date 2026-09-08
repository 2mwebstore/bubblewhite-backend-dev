package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterProductRoutes(api *gin.RouterGroup, c *Container) {
	// Public — storefront catalog
	public := api.Group("/products")
	public.GET("", c.Product.List)
	public.GET("/:id", c.Product.GetByID)
	public.GET("/:id/related", c.Product.Related)

	// Admin — protected by JWT + specific permission per action
	admin := api.Group("/admin/products")
	admin.Use(middlewares.AuthMiddleware())
	admin.Use(c.AdminRateLimiter.Middleware())
	admin.POST("", middlewares.RequirePermission(c.DB, "product.create"), c.Product.Create)
	admin.PUT("/:id", middlewares.RequirePermission(c.DB, "product.update"), c.Product.Update)
	admin.DELETE("/:id", middlewares.RequirePermission(c.DB, "product.delete"), c.Product.Delete)
}
