package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterCategoryRoutes(api *gin.RouterGroup, c *Container) {
	public := api.Group("/categories")
	public.GET("", c.Category.List)
	public.GET("/:id", c.Category.GetByID)

	admin := api.Group("/admin/categories")
	admin.Use(middlewares.AuthMiddleware())
	admin.Use(c.AdminRateLimiter.Middleware())
	admin.GET("", middlewares.RequirePermission(c.DB, "category.view"), c.Category.ListAdmin)
	admin.POST("", middlewares.RequirePermission(c.DB, "category.create"), c.Category.Create)
	admin.PUT("/:id", middlewares.RequirePermission(c.DB, "category.update"), c.Category.Update)
	admin.DELETE("/:id", middlewares.RequirePermission(c.DB, "category.delete"), c.Category.Delete)
}
