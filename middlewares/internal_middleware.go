package middlewares

import (
	"crypto/subtle"

	"bubblewhite-backend/config"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

// RequireInternalSecret protects routes meant only for other services in
// this deployment (not browsers, not the admin/customer JWT systems) — the
// caller must send the exact same value configured via INTERNAL_API_SECRET
// in the X-Internal-Secret header. Fails closed: if the secret isn't
// configured on this server at all, every request is rejected rather than
// silently treated as authorized.
func RequireInternalSecret() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := config.Get().InternalAPISecret
		if expected == "" {
			utils.InternalError(c, "internal API secret is not configured on this server")
			c.Abort()
			return
		}

		provided := c.GetHeader("X-Internal-Secret")
		// Constant-time comparison — this is a real access-control check,
		// not just an internal sanity check, so it shouldn't leak timing
		// information about how much of the secret matched.
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			utils.Unauthorized(c, "invalid internal secret")
			c.Abort()
			return
		}

		c.Next()
	}
}
