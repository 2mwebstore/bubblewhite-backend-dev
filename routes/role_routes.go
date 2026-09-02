package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterRoleRoutes(api *gin.RouterGroup, c *Container) {
	admin := api.Group("/admin/roles")
	admin.Use(middlewares.AuthMiddleware())

	admin.GET("", middlewares.RequirePermission(c.DB, "role.view"), c.Role.List)
	admin.POST("", middlewares.RequirePermission(c.DB, "role.create"), c.Role.Create)
	admin.PUT("/:id", middlewares.RequirePermission(c.DB, "role.update"), c.Role.Update)
	admin.DELETE("/:id", middlewares.RequirePermission(c.DB, "role.delete"), c.Role.Delete)

	// Lets the admin UI render a checklist of every grantable permission
	// when creating/editing a role.
	api.GET("/admin/permissions", middlewares.AuthMiddleware(), middlewares.RequirePermission(c.DB, "role.view"), c.Role.ListPermissions)
}
