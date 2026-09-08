package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterCustomerRoutes(api *gin.RouterGroup, c *Container) {
	// Public — register/login for storefront shoppers.
	api.POST("/customer/register", c.LoginRateLimiter.Middleware(), c.Customer.Register)
	api.POST("/customer/login", c.LoginRateLimiter.Middleware(), c.Customer.Login)
	api.POST("/customer/auth/google", c.LoginRateLimiter.Middleware(), c.Customer.GoogleLogin)
	api.POST("/customer/auth/facebook", c.LoginRateLimiter.Middleware(), c.Customer.FacebookLogin)

	// Same LoginRateLimiter as every other auth endpoint — arguably even
	// more important here, since RequestOTPBySMS costs real money per
	// call through Plasgate. A real customer requests a code once, maybe
	// twice if the first didn't arrive; this is squarely aimed at
	// someone trying to either run up an SMS bill or spam a phone
	// number that isn't theirs with unwanted messages.
	api.POST("/customer/otp/request", c.LoginRateLimiter.Middleware(), c.Customer.RequestOTP)
	api.POST("/customer/otp/request-sms", c.LoginRateLimiter.Middleware(), c.Customer.RequestOTPBySMS)
	api.POST("/customer/otp/verify", c.LoginRateLimiter.Middleware(), c.Customer.VerifyOTP)

	me := api.Group("/customer/me")
	me.Use(middlewares.CustomerAuthMiddleware())
	me.GET("", c.Customer.Me)
	me.PUT("", c.Customer.UpdateProfile)
	// Same LoginRateLimiter as the login endpoint itself — a stolen/
	// guessed customer JWT could otherwise be used to brute-force the
	// current password here without ever touching /customer/login.
	me.POST("/change-password", c.LoginRateLimiter.Middleware(), c.Customer.ChangePassword)

	cart := api.Group("/customer/cart")
	cart.Use(middlewares.CustomerAuthMiddleware())
	cart.GET("", c.Cart.List)
	cart.POST("", c.Cart.Add)
	cart.PUT("/:id", c.Cart.UpdateQuantity)
	cart.DELETE("/:id", c.Cart.Remove)
	cart.DELETE("", c.Cart.Clear)
}
