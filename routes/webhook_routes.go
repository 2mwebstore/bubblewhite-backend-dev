package routes

import (
	"github.com/gin-gonic/gin"
)

// RegisterWebhookRoutes — deliberately NOT behind AuthMiddleware.
// External services (PPCBank, Telegram) POST here directly and have no
// way to obtain our JWT credentials. Each handler is responsible for its
// own trust model instead — see PPCBankWebhookController.Notify, which
// never trusts the payload alone and always independently re-verifies
// against PPCBank's real API. TelegramBotController.Webhook's trust model
// is different but equally deliberate: it only ever acts on a /start
// <token> whose token it can find in its own OtpRequest table, so an
// arbitrary POST to this URL that isn't genuinely from Telegram can't do
// anything more than that.
func RegisterWebhookRoutes(api *gin.RouterGroup, c *Container) {
	webhooks := api.Group("/webhooks")
	webhooks.POST("/ppcbank", c.PPCBankWebhook.Notify)
	webhooks.POST("/telegram", c.TelegramBot.Webhook)
}
