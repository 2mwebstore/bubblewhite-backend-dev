package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(api *gin.RouterGroup, c *Container) {
	auth := api.Group("/auth")
	auth.POST("/login", c.Auth.Login)

	me := api.Group("/admin/me")
	me.Use(middlewares.AuthMiddleware())
	me.GET("", c.Auth.Me)
	me.POST("/change-password", c.Auth.ChangePassword)
}
