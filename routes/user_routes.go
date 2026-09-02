package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(api *gin.RouterGroup, c *Container) {
	admin := api.Group("/admin/users")
	admin.Use(middlewares.AuthMiddleware())

	admin.GET("", middlewares.RequirePermission(c.DB, "user.view"), c.User.List)
	admin.GET("/:id", middlewares.RequirePermission(c.DB, "user.view"), c.User.GetByID)
	admin.POST("", middlewares.RequirePermission(c.DB, "user.create"), c.User.Create)
	admin.PUT("/:id", middlewares.RequirePermission(c.DB, "user.update"), c.User.Update)
	admin.DELETE("/:id", middlewares.RequirePermission(c.DB, "user.delete"), c.User.Delete)

	// Assigning a role IS how permissions get granted to a user — kept as
	// its own permission (user.manage) so it can be restricted separately
	// from ordinary profile edits.
	admin.PATCH("/:id/role", middlewares.RequirePermission(c.DB, "user.manage"), c.User.AssignRole)

	// Admin-initiated password reset for another user — gated on user.update
	// since it's a normal part of managing an account, not a separate
	// elevated permission.
	admin.PATCH("/:id/password", middlewares.RequirePermission(c.DB, "user.update"), c.User.ResetPassword)
}
