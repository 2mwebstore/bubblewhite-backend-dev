package controllers

import (
	"strconv"
	"strings"

	"bubblewhite-backend/services"

	"github.com/gin-gonic/gin"
)

type TelegramBotController struct {
	Otp *services.OtpService
}

func NewTelegramBotController(otp *services.OtpService) *TelegramBotController {
	return &TelegramBotController{Otp: otp}
}

// telegramUpdate mirrors only the fields this handler actually needs from
// Telegram's Update object — see core.telegram.org/bots/api#update. Every
// incoming message webhook Telegram calls has this shape; most of it
// (edited messages, channel posts, callback queries, etc.) is simply
// never populated for a plain text message and is left out here rather
// than modeled unused.
type telegramUpdate struct {
	Message struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// Webhook receives every update Telegram sends this bot — configured via
// Telegram's setWebhook API to point at this endpoint. Deliberately always
// responds 200 regardless of outcome: Telegram interprets a non-200 as
// "retry this update later" and will keep resending it, which would just
// repeat the same (by then likely already-consumed or expired)
// verification attempt rather than fix anything.
func (ctrl *TelegramBotController) Webhook(c *gin.Context) {
	var update telegramUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(200, gin.H{"ok": true})
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	if !strings.HasPrefix(text, "/start ") {
		// Not a deep-link /start — some other message to the bot (a
		// stray "hi", an old command). Nothing for this feature to do
		// with it; still acknowledge so Telegram doesn't retry.
		c.JSON(200, gin.H{"ok": true})
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(text, "/start "))
	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)

	// Errors here are deliberately not surfaced back to Telegram/the
	// customer through this endpoint — there's no UI on the Telegram
	// side of this webhook to show one on. If CompleteTelegramLink fails
	// (an expired or already-used token, for instance), the customer
	// simply never receives a code in this chat; when they then try to
	// verify on the website, OtpService.VerifyOTP naturally rejects it
	// with a clear "expired or not requested" message, which is what
	// actually surfaces the problem back to them.
	_ = ctrl.Otp.CompleteTelegramLink(token, chatID)

	c.JSON(200, gin.H{"ok": true})
}
