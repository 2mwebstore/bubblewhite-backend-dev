package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterPaymentMethodRoutes(api *gin.RouterGroup, c *Container) {
	// Public — the storefront's checkout selector reads its options from
	// here, same pattern as GET /api/settings.
	api.GET("/payment-methods", c.PaymentMethod.ListPublic)

	admin := api.Group("/admin/payment-methods")
	admin.Use(middlewares.AuthMiddleware())
	admin.GET("", middlewares.RequirePermission(c.DB, "payment_method.view"), c.PaymentMethod.List)
	admin.GET("/:id", middlewares.RequirePermission(c.DB, "payment_method.view"), c.PaymentMethod.GetByID)
	admin.PUT("/:id", middlewares.RequirePermission(c.DB, "payment_method.update"), c.PaymentMethod.Update)
}
