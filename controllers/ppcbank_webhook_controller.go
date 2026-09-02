package controllers

import (
	"log"

	"bubblewhite-backend/models"
	"bubblewhite-backend/services"

	"github.com/gin-gonic/gin"
)

type PPCBankWebhookController struct {
	Service *services.OrderService
}

func NewPPCBankWebhookController(s *services.OrderService) *PPCBankWebhookController {
	return &PPCBankWebhookController{Service: s}
}

type ppcbankWebhookPayload struct {
	BillNumber    string `json:"billNumber"`
	PaymentStatus string `json:"paymentStatus"`
}

// POST /api/webhooks/ppcbank — PPCBank's "Notify KHQR Payment Result"
// callback. Deliberately does NOT trust payload.PaymentStatus on its own —
// PPCBank's docs describe no signature or authentication mechanism on this
// webhook, so anyone who discovered this URL could POST a forged "Success"
// payload. The payload is only used to learn WHICH order to check; the
// actual payment confirmation always comes from independently calling
// CheckPPCBankPaymentStatus (via OrderService.VerifyPPCBankPayment) — the
// exact same real API call the admin's manual re-check button uses.
//
// Response shape is PPCBank's own required contract, not our normal API
// envelope (utils.OK) — must respond with exactly {"code":200,"message":
// "SUCCESS"} or PPCBank may treat the notification as failed.
func (ctrl *PPCBankWebhookController) Notify(c *gin.Context) {
	var payload ppcbankWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.BillNumber == "" {
		log.Printf("ppcbank webhook: malformed or missing billNumber in payload: %v", err)
		// Still acknowledge with PPCBank's expected 200 contract — a
		// malformed webhook has nothing to look up and no way to retry
		// on our end (PPCBank's docs say they don't retry either), so
		// there is nothing further useful this response could do besides
		// getting logged, which already happened above.
		c.JSON(200, gin.H{"code": 200, "message": "SUCCESS"})
		return
	}

	orderID, err := models.ParseOrderReference(payload.BillNumber)
	if err != nil {
		log.Printf("ppcbank webhook: billNumber %q does not match our reference format: %v", payload.BillNumber, err)
		c.JSON(200, gin.H{"code": 200, "message": "SUCCESS"})
		return
	}

	if _, err := ctrl.Service.VerifyPPCBankPayment(orderID); err != nil {
		// Logged, but still acknowledged — per PPCBank's docs there's no
		// retry mechanism on their side regardless, and the admin's
		// manual re-check button (same VerifyPPCBankPayment call) remains
		// available as the real fallback if this independent check
		// itself failed transiently.
		log.Printf("ppcbank webhook: VerifyPPCBankPayment failed for order %d: %v", orderID, err)
	}

	c.JSON(200, gin.H{"code": 200, "message": "SUCCESS"})
}
