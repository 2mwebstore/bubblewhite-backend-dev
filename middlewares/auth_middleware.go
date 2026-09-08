package middlewares

import (
	"strings"

	"bubblewhite-backend/models"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	ctxUserID = "authUserID"
	ctxEmail  = "authEmail"
	ctxRole   = "authRoleSlug"
)

// AuthMiddleware validates the Bearer token on the Authorization header and
// injects the caller's user ID / email / role slug into the request context.
// It does NOT check permissions — pair with RequirePermission for that.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			utils.Unauthorized(c, "missing or malformed Authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			utils.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxEmail, claims.Email)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// RequirePermission checks that the authenticated user's role grants the
// given permission slug (see models.Role.HasPermission / seed.PermissionCatalog).
// Must run after AuthMiddleware. The "*" permission (superadmin) always passes.
func RequirePermission(db *gorm.DB, slug string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleSlug, ok := c.Get(ctxRole)
		if !ok {
			utils.Unauthorized(c, "authentication required")
			c.Abort()
			return
		}

		var role models.Role
		if err := db.Where("slug = ?", roleSlug).First(&role).Error; err != nil {
			utils.Forbidden(c, "role not found")
			c.Abort()
			return
		}

		if !role.HasPermission(slug) {
			utils.Forbidden(c, "you don't have permission to perform this action")
			c.Abort()
			return
		}

		c.Next()
	}
}

// CurrentUserID reads the authenticated user's ID out of context.
func CurrentUserID(c *gin.Context) uint {
	v, _ := c.Get(ctxUserID)
	id, _ := v.(uint)
	return id
}

// CurrentUserEmail reads the authenticated user's email out of context —
// used as the ActorName for admin audit log entries (see
// controllers/audit_log helpers), so recording who did something doesn't
// need an extra database lookup on every single mutation just to get a
// display name; the email already carried in the token is enough.
func CurrentUserEmail(c *gin.Context) string {
	v, _ := c.Get(ctxEmail)
	email, _ := v.(string)
	return email
}

// CurrentRoleSlug reads the authenticated user's role slug out of context.
func CurrentRoleSlug(c *gin.Context) string {
	v, _ := c.Get(ctxRole)
	slug, _ := v.(string)
	return slug
}
