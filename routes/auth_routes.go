package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(api *gin.RouterGroup, c *Container) {
	auth := api.Group("/auth")
	auth.POST("/login", c.LoginRateLimiter.Middleware(), c.Auth.Login)

	me := api.Group("/admin/me")
	me.Use(middlewares.AuthMiddleware())
	me.Use(c.AdminRateLimiter.Middleware())
	me.GET("", c.Auth.Me)
	// Same LoginRateLimiter as the login endpoint itself — a stolen/
	// guessed staff JWT could otherwise be used to brute-force the
	// current password on this endpoint without ever needing to touch
	// /auth/login at all.
	me.POST("/change-password", c.LoginRateLimiter.Middleware(), c.Auth.ChangePassword)
}
