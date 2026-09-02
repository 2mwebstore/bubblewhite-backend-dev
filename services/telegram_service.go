package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"bubblewhite-backend/config"
	"bubblewhite-backend/models"
)

// sendTelegramMessage posts a plain message to the configured Telegram
// group via the Bot API. Deliberately never returns an error — a failed
// or slow notification must NEVER block or fail the actual checkout/
// payment flow that triggered it. Silently no-ops if not configured,
// since notifications are an enhancement, not something checkout depends
// on (same philosophy as Telegram integration everywhere else in this
// codebase's third-party API handling: log failures, don't let them
// propagate to the customer-facing response).
func sendTelegramMessage(text string) {
	botToken := config.Get().TelegramBotToken
	chatID := config.Get().TelegramChatID
	if botToken == "" || chatID == "" {
		// Logged — without this, "not configured" and "configured but
		// silently failing" look identical from the outside: no message
		// ever arrives, and nothing anywhere explains why. This exact
		// class of silent-skip has bitten this project more than once
		// already (PPCBank checkout initiation, payment verification).
		log.Println("telegram: TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID not configured — skipping notification")
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("telegram: failed to encode notification payload: %v", err)
		return
	}

	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("telegram: failed to send notification: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Telegram's error responses are informative JSON (e.g. "chat not
		// found", "bot was blocked by the user", "Unauthorized" for a bad
		// token) — reading the body, not just the status code, is what
		// actually tells you WHICH of those it is.
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("telegram: sendMessage failed, status=%d body=%s", resp.StatusCode, string(respBody))
		return
	}

	log.Println("telegram: order-paid notification sent successfully")
}

// NotifyOrderPaid sends a formatted order-paid alert to the configured
// Telegram group — call this from every code path that transitions an
// order's PaymentStatus to "paid" (OrderService.Checkout,
// VerifyBakongPayment, VerifyPPCBankPayment). Runs in its own goroutine —
// the actual HTTP call to Telegram has real network latency, and nothing
// here should ever add delay to the customer-facing checkout response
// that's already succeeded by the time this is called.
func NotifyOrderPaid(order *models.Order) {
	// The order reference becomes a clickable link straight to its admin
	// review page when FrontendBaseURL is configured — falls back to
	// plain, non-clickable text otherwise, since a link built from an
	// empty base URL would just be a broken "/admin/orders/42" with no
	// domain, which is worse than no link at all.
	reference := order.Reference()
	if config.Get().FrontendBaseURL != "" {
		reviewURL := fmt.Sprintf("%s/admin/orders/%d", config.Get().FrontendBaseURL, order.ID)
		reference = fmt.Sprintf(`<a href="%s">%s</a>`, reviewURL, order.Reference())
	}

	text := fmt.Sprintf(
		"✅ <b>ការទូទាត់ជោគជ័យ</b>\n\n"+
			"<b>លេខការបញ្ជាទិញ:</b> %s\n"+
			"<b>វិធីទូទាត់:</b> %s\n"+
			"<b>សរុប:</b> $%.2f\n"+
			"<b>លេខទូរស័ព្ទ:</b> %s\n"+
			"<b>អាសយដ្ឋាន:</b> %s",
		reference,
		paymentMethodLabel(order.PaymentMethod),
		order.Total,
		order.Phone,
		order.Address,
	)
	// Invoice (the bank's own reference number — see Order.Invoice's doc
	// comment) is only populated for PPCBank payments once verified, so
	// this line is appended conditionally rather than always shown blank
	// for Bakong/cash orders.
	if order.Invoice != "" {
		text += fmt.Sprintf("\n<b>លេខតម្រុយធនាគារ:</b> %s", order.Invoice)
	}
	go sendTelegramMessage(text)
}

func paymentMethodLabel(code string) string {
	switch code {
	case models.PaymentMethodBakong:
		return "Bakong KHQR"
	case models.PaymentMethodPPCBank:
		return "PPCBank KHQR"
	case models.PaymentMethodCash:
		return "សាច់ប្រាក់"
	default:
		return code
	}
}
