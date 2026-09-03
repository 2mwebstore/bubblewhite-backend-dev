package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterAdminOrderRoutes(api *gin.RouterGroup, c *Container) {
	orders := api.Group("/admin/orders")
	orders.Use(middlewares.AuthMiddleware())
	orders.GET("", middlewares.RequirePermission(c.DB, "order.view"), c.AdminOrder.List)
	orders.GET("/:id", middlewares.RequirePermission(c.DB, "order.view"), c.AdminOrder.GetByID)
	orders.PATCH("/:id/status", middlewares.RequirePermission(c.DB, "order.manage"), c.AdminOrder.UpdateStatus)
	orders.POST("/:id/verify-payment", middlewares.RequirePermission(c.DB, "order.manage"), c.AdminOrder.VerifyPayment)
}
