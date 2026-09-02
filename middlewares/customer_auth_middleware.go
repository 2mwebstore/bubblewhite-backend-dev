package middlewares

import (
	"strings"

	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

const ctxCustomerID = "authCustomerID"

// CustomerAuthMiddleware validates the Bearer token as a CUSTOMER token
// (see utils.ParseCustomerToken) — entirely separate from AuthMiddleware,
// which validates admin/staff tokens. A customer token will never pass
// AuthMiddleware and vice versa, since they're signed with different claim
// shapes and parsed with different functions.
func CustomerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			utils.Unauthorized(c, "missing or malformed Authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := utils.ParseCustomerToken(tokenString)
		if err != nil {
			utils.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(ctxCustomerID, claims.CustomerID)
		c.Next()
	}
}

// CurrentCustomerID reads the authenticated customer's ID out of context.
func CurrentCustomerID(c *gin.Context) uint {
	v, _ := c.Get(ctxCustomerID)
	id, _ := v.(uint)
	return id
}
