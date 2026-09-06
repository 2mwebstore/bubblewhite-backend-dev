package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterContactRoutes(api *gin.RouterGroup, c *Container) {
	// Public — the storefront's Contact page form posts here.
	api.POST("/contact", c.ContactRateLimiter.Middleware(), c.Contact.Submit)

	admin := api.Group("/admin/contacts")
	admin.Use(middlewares.AuthMiddleware())
	admin.GET("", middlewares.RequirePermission(c.DB, "contact.view"), c.Contact.List)
	admin.PATCH("/:id/read", middlewares.RequirePermission(c.DB, "contact.manage"), c.Contact.MarkRead)
	admin.DELETE("/:id", middlewares.RequirePermission(c.DB, "contact.manage"), c.Contact.Delete)
}
