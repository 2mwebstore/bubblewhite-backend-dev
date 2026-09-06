package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodySize wraps the request body in http.MaxBytesReader, which causes
// any read past the given limit to fail rather than allocating unbounded
// memory for it. Applied globally with a limit generous enough for a
// high-resolution product photo upload (the largest legitimate payload
// this API accepts) — everything else (JSON API calls, login, checkout)
// is only ever a few KB, so this never affects real usage.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
