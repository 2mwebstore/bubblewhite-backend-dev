package routes

import (
	"github.com/gin-gonic/gin"
)

// RegisterWebhookRoutes — deliberately NOT behind AuthMiddleware.
// External services (PPCBank) POST here directly and have no way to
// obtain our JWT credentials. Each handler is responsible for its own
// trust model instead — see PPCBankWebhookController.Notify, which never
// trusts the payload alone and always independently re-verifies against
// PPCBank's real API.
func RegisterWebhookRoutes(api *gin.RouterGroup, c *Container) {
	webhooks := api.Group("/webhooks")
	webhooks.POST("/ppcbank", c.PPCBankWebhook.Notify)
}
