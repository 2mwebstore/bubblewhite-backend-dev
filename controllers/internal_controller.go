package controllers

import (
	"bubblewhite-backend/config"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

// InternalController holds endpoints meant only for OTHER SERVICES in this
// deployment to call (currently just the Nuxt frontend), never a browser
// or mobile client directly. Protected by a shared secret (see
// middlewares.RequireInternalSecret), not the customer/admin JWT systems —
// this isn't a user-facing concept at all.
type InternalController struct{}

func NewInternalController() *InternalController {
	return &InternalController{}
}

// GET /api/internal/bakong-token
// Returns this backend's current cached Bakong Open API token — reusing
// GetCachedBakongAPIToken()'s existing in-memory + database caching rather
// than making the frontend maintain its own separate cache and its own
// separate calls to Bakong's renew_token (which would otherwise silently
// double up on Bakong's shared daily request quota between the two
// services for zero benefit).
func (ctrl *InternalController) GetBakongToken(c *gin.Context) {
	if config.Get().InternalAPISecret == "" {
		// Fails closed rather than silently allowing unauthenticated
		// access — an unset secret must never be treated as "no auth
		// required."
		utils.InternalError(c, "internal API secret is not configured on this server")
		return
	}

	token, err := services.GetCachedBakongAPIToken()
	if err != nil {
		utils.InternalError(c, "failed to obtain Bakong token: "+err.Error())
		return
	}
	utils.OK(c, gin.H{"token": token})
}
