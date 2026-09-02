package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterInternalRoutes(api *gin.RouterGroup, c *Container) {
	internal := api.Group("/internal")
	internal.Use(middlewares.RequireInternalSecret())
	internal.GET("/bakong-token", c.Internal.GetBakongToken)
}
