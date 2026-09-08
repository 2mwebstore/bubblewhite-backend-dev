package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// PlasgateSMSService sends SMS via Plasgate — Cambodia's largest SMS
// gateway, authorized by all major local carriers (Smart, Cellcard,
// Metfone, SeaTel, qb, CooTel). Used here as the fallback channel for OTP
// delivery when a customer doesn't have Telegram linked — see
// OtpService, which tries the free Telegram channel first and only
// falls back to this paid one when it has to.
//
// Every field and endpoint here is taken directly from Plasgate's own
// REST API documentation (cloud.plasgate.com/support/rest), not guessed —
// this is a real, billable SMS integration, and getting the request
// shape wrong here means either OTPs silently fail to send, or (worse)
// send incorrectly without any indication why.
type PlasgateSMSService struct {
	privateKey string
	secretKey  string
	senderID   string
}

func NewPlasgateSMSService(privateKey, secretKey, senderID string) *PlasgateSMSService {
	return &PlasgateSMSService{privateKey: privateKey, secretKey: secretKey, senderID: senderID}
}

var ErrPlasgateNotConfigured = errors.New("sms sending is not configured")

type plasgateSendResponse struct {
	// Plasgate's docs don't specify field names for the send response
	// body precisely — this is deliberately loose (map, checked below
	// for an HTTP-level failure only) rather than a fully-typed struct
	// that might silently drop fields their API actually returns.
	Error string `json:"error"`
}

// SendSMS sends a single SMS via Plasgate's REST API
// (POST https://cloudapi.plasgate.com/rest/send). Authentication is
// split across two places per their docs, not a single header or body
// field: private_key as a URL query parameter, and the secret key as a
// custom X-Secret HEADER — an unusual split worth calling out explicitly
// since it's easy to get wrong by putting both in the same place.
func (p *PlasgateSMSService) SendSMS(to, content string) error {
	if p.privateKey == "" || p.secretKey == "" {
		return ErrPlasgateNotConfigured
	}

	body, err := json.Marshal(map[string]string{
		"sender":  p.senderID,
		"to":      to,
		"content": content,
	})
	if err != nil {
		return fmt.Errorf("encoding plasgate request: %w", err)
	}

	url := "https://cloudapi.plasgate.com/rest/send?private_key=" + p.privateKey
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building plasgate request: %w", err)
	}
	req.Header.Set("X-Secret", p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling plasgate: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var parsed plasgateSendResponse
		_ = json.Unmarshal(respBody, &parsed)
		if parsed.Error != "" {
			return fmt.Errorf("plasgate rejected the message: %s", parsed.Error)
		}
		return fmt.Errorf("plasgate returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendOTP wraps SendSMS with the actual OTP message text and Plasgate's
// masking syntax (#ma#...#ma#) around the code itself — per their docs,
// masked text still reaches the recipient's device in full, but shows as
// asterisks in Plasgate's own transaction reports. There's no reason a
// short-lived, single-use code needs to sit in a third party's visible
// logs even though it's already expired by the time anyone would look.
func (p *PlasgateSMSService) SendOTP(phone, code string) error {
	content := fmt.Sprintf("BubbleWhite: លេខកូដផ្ទៀងផ្ទាត់របស់អ្នកគឺ #ma#%s#ma#។ សូមកុំប្រាប់លេខកូដនេះទៅអ្នកដទៃ។", code)
	return p.SendSMS(phone, content)
}
