package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterOrderRoutes(api *gin.RouterGroup, c *Container) {
	orders := api.Group("/customer/orders")
	orders.Use(middlewares.CustomerAuthMiddleware())
	orders.POST("", c.CheckoutRateLimiter.Middleware(), c.Order.Checkout)
	orders.POST("/ppcbank/initiate", c.CheckoutRateLimiter.Middleware(), c.Order.InitiatePPCBankCheckout)
	orders.GET("/ppcbank/status", c.Order.PPCBankReturnStatus)
	orders.GET("", c.Order.List)
	orders.GET("/:id", c.Order.GetByID)
}
