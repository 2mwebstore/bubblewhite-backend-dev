package routes

import (
	"bubblewhite-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterAuditLogRoutes(api *gin.RouterGroup, c *Container) {
	admin := api.Group("/admin/audit-logs")
	admin.Use(middlewares.AuthMiddleware())
	admin.Use(c.AdminRateLimiter.Middleware())
	admin.GET("/staff", middlewares.RequirePermission(c.DB, "audit.view"), c.AuditLog.ListStaff)
	admin.GET("/staff/filters", middlewares.RequirePermission(c.DB, "audit.view"), c.AuditLog.StaffFilterOptions)
	admin.GET("/customers", middlewares.RequirePermission(c.DB, "audit.view"), c.AuditLog.ListCustomers)
	admin.GET("/customers/filters", middlewares.RequirePermission(c.DB, "audit.view"), c.AuditLog.CustomerFilterOptions)
	admin.DELETE("/staff", middlewares.RequirePermission(c.DB, "audit.manage"), c.AuditLog.CleanupStaff)
	admin.DELETE("/customers", middlewares.RequirePermission(c.DB, "audit.manage"), c.AuditLog.CleanupCustomers)
	admin.POST("/staff/delete-selected", middlewares.RequirePermission(c.DB, "audit.manage"), c.AuditLog.DeleteSelectedStaff)
	admin.POST("/customers/delete-selected", middlewares.RequirePermission(c.DB, "audit.manage"), c.AuditLog.DeleteSelectedCustomers)
}
