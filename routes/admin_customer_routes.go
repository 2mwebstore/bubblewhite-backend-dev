package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterAdminCustomerRoutes(api *gin.RouterGroup, c *Container) {
	customers := api.Group("/admin/customers")
	customers.Use(middlewares.AuthMiddleware())
	customers.GET("", middlewares.RequirePermission(c.DB, "customer.view"), c.AdminCustomer.List)
	customers.GET("/:id", middlewares.RequirePermission(c.DB, "customer.view"), c.AdminCustomer.GetByID)
	customers.GET("/:id/orders", middlewares.RequirePermission(c.DB, "customer.view"), c.AdminOrder.ListByCustomer)
	customers.PATCH("/:id/password", middlewares.RequirePermission(c.DB, "customer.manage"), c.AdminCustomer.ResetPassword)
	customers.PATCH("/:id/active", middlewares.RequirePermission(c.DB, "customer.manage"), c.AdminCustomer.SetActive)
}
