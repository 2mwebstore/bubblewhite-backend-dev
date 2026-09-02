package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterCustomerRoutes(api *gin.RouterGroup, c *Container) {
	// Public — register/login for storefront shoppers.
	api.POST("/customer/register", c.Customer.Register)
	api.POST("/customer/login", c.Customer.Login)

	me := api.Group("/customer/me")
	me.Use(middlewares.CustomerAuthMiddleware())
	me.GET("", c.Customer.Me)
	me.PUT("", c.Customer.UpdateProfile)
	me.POST("/change-password", c.Customer.ChangePassword)

	cart := api.Group("/customer/cart")
	cart.Use(middlewares.CustomerAuthMiddleware())
	cart.GET("", c.Cart.List)
	cart.POST("", c.Cart.Add)
	cart.PUT("/:id", c.Cart.UpdateQuantity)
	cart.DELETE("/:id", c.Cart.Remove)
	cart.DELETE("", c.Cart.Clear)
}
