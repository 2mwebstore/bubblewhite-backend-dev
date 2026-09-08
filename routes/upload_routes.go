package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterUploadRoutes(api *gin.RouterGroup, c *Container) {
	admin := api.Group("/admin/uploads")
	admin.Use(middlewares.AuthMiddleware())
	admin.Use(c.AdminRateLimiter.Middleware())

	admin.POST("/products", middlewares.RequirePermission(c.DB, "product.update"), c.Upload.UploadProductImage)
	admin.POST("/categories", middlewares.RequirePermission(c.DB, "category.update"), c.Upload.UploadCategoryImage)
	admin.POST("/site", middlewares.RequirePermission(c.DB, "settings.update"), c.Upload.UploadSiteImage)
	admin.POST("/banners", middlewares.RequirePermission(c.DB, "banner.create"), c.Upload.UploadBannerImage)
	admin.POST("/payment-methods", middlewares.RequirePermission(c.DB, "payment_method.update"), c.Upload.UploadPaymentMethodImage)

	admin.DELETE("/products", middlewares.RequirePermission(c.DB, "product.update"), c.Upload.DeleteProductImage)
	admin.DELETE("/categories", middlewares.RequirePermission(c.DB, "category.update"), c.Upload.DeleteCategoryImage)
	admin.DELETE("/site", middlewares.RequirePermission(c.DB, "settings.update"), c.Upload.DeleteSiteImage)
	admin.DELETE("/banners", middlewares.RequirePermission(c.DB, "banner.update"), c.Upload.DeleteBannerImage)
	admin.DELETE("/payment-methods", middlewares.RequirePermission(c.DB, "payment_method.update"), c.Upload.DeletePaymentMethodImage)
}
