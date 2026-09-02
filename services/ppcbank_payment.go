package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"bubblewhite-backend/config"
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

// Package-level token cache, same two-tier (in-memory + database) pattern
// as bakong_payment.go — see that file for the full reasoning. Reuses this
// package's shared httpClient and FlexibleNumber type (both already
// defined in bakong_payment.go).
var (
	ppcbankTokenMu    sync.Mutex
	ppcbankDB         *gorm.DB
	ppcbankMemToken   string
	ppcbankMemTokenAt time.Time
	ppcbankTokenTTL   = 12 * time.Hour
	ErrPPCBankNotSet  = errors.New("PPCBANK_MERCHANT_CODE/PPCBANK_PASSWORD is not configured")
)

// SetPPCBankDB wires the database connection this package uses to persist
// the PPCBank API token — called once from routes.Build() at boot.
func SetPPCBankDB(db *gorm.DB) {
	ppcbankDB = db
}

func loadCachedPPCBankToken() (string, bool) {
	if ppcbankMemToken != "" && time.Since(ppcbankMemTokenAt) < ppcbankTokenTTL {
		return ppcbankMemToken, true
	}
	if ppcbankDB == nil {
		return "", false
	}
	var row models.PPCBankToken
	if err := ppcbankDB.First(&row).Error; err != nil {
		return "", false
	}
	if time.Since(row.FetchedAt) >= ppcbankTokenTTL {
		return "", false
	}
	ppcbankMemToken = row.Token
	ppcbankMemTokenAt = row.FetchedAt
	log.Println("ppcbank: reusing cached token from database (process restarted since it was last fetched)")
	return row.Token, true
}

func saveCachedPPCBankToken(token string) {
	ppcbankMemToken = token
	ppcbankMemTokenAt = time.Now()
	if ppcbankDB == nil {
		return
	}
	var row models.PPCBankToken
	if err := ppcbankDB.First(&row).Error; err != nil {
		ppcbankDB.Create(&models.PPCBankToken{Token: token, FetchedAt: ppcbankMemTokenAt})
		return
	}
	row.Token = token
	row.FetchedAt = ppcbankMemTokenAt
	ppcbankDB.Save(&row)
}

func getPPCBankToken(forceRefresh bool) (string, error) {
	ppcbankTokenMu.Lock()
	defer ppcbankTokenMu.Unlock()

	if !forceRefresh {
		if cached, ok := loadCachedPPCBankToken(); ok {
			return cached, nil
		}
	}

	merchantCode := config.Get().PPCBankMerchantCode
	password := config.Get().PPCBankPassword
	if merchantCode == "" || password == "" {
		return "", ErrPPCBankNotSet
	}

	log.Println("ppcbank: fetching a new API token via security_check")

	body, _ := json.Marshal(map[string]string{"merchantCode": merchantCode, "password": password})
	req, err := http.NewRequest(http.MethodPost, config.Get().PPCBankAPIBaseURL+"/security_check", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build PPCBank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Explicit User-Agent — without this, Go's net/http sends the stock
	// default "Go-http-client/1.1", which is a well-known signature many
	// firewalls/WAFs treat as automated/bot traffic and silently drop
	// rather than respond to (the connection just hangs until timeout,
	// with no clean HTTP rejection — exactly the symptom observed here).
	req.Header.Set("User-Agent", "BubbleWhite-Backend/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("ppcbank: security_check request failed: %v", err)
		return "", fmt.Errorf("failed to reach PPCBank API: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Header ppcbankHeader `json:"header"`
		Body   struct {
			Token string `json:"token"`
		} `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("ppcbank: failed to decode security_check response: %v", err)
		return "", fmt.Errorf("failed to parse PPCBank response: %w", err)
	}

	// PPCBank's docs are explicit that HTTP status is 200 even on logical
	// failure — the actual result always lives in header.result, never
	// inferred from the HTTP status code.
	if !result.Header.Result || result.Body.Token == "" {
		log.Printf("ppcbank: security_check failed, resultCode=%s message=%q", result.Header.ResultCode, result.Header.ResultMessage)
		return "", fmt.Errorf("ppcbank: %s", result.Header.ResultMessage)
	}

	saveCachedPPCBankToken(result.Body.Token)
	log.Println("ppcbank: new token cached (in-memory + database)")
	return result.Body.Token, nil
}

func invalidatePPCBankToken() {
	ppcbankTokenMu.Lock()
	defer ppcbankTokenMu.Unlock()
	ppcbankMemToken = ""
	if ppcbankDB != nil {
		ppcbankDB.Where("id > 0").Delete(&models.PPCBankToken{})
	}
}

type ppcbankHeader struct {
	Result        bool   `json:"result"`
	ResultCode    string `json:"resultCode"`
	ResultMessage string `json:"resultMessage"`
}

// PPCBankPaymentDetail mirrors the `body` object returned by Check KHQR
// Payment Status. TransactionAmount uses FlexibleNumber (defined in
// bakong_payment.go) defensively — Bakong's API was documented as one
// type and actually returned another in practice, so the same caution
// applies here even though PPCBank's docs list it as a plain Double.
// FlexibleString decodes from either a JSON string or a JSON number,
// always producing a plain Go string — for fields like referenceNo that
// PPCBank's own docs type as String, but which have been observed
// returned as a raw JSON number in live testing (confirmed directly
// against the real API: {"referenceNo": 376647790}, not "376647790").
// Same class of documented-type-vs-actual-type mismatch as
// FlexibleNumber above; referenceNo stays a string here (not a
// FlexibleNumber) since it's an opaque identifier, not a value anything
// does arithmetic on.
type FlexibleString string

func (f *FlexibleString) UnmarshalJSON(data []byte) error {
	*f = FlexibleString(strings.Trim(string(data), `"`))
	return nil
}

type PPCBankPaymentDetail struct {
	WithdrawalAccountNo     string         `json:"withdrawalAccountNo"`
	SenderBankCode          string         `json:"senderBankCode"`
	SenderBankName          string         `json:"senderBankName"`
	SenderName              string         `json:"senderName"`
	TransactionAmount       FlexibleNumber `json:"transactionAmount"`
	TransactionCurrencyCode string         `json:"transactionCurrencyCode"`
	BillStatusCode          string         `json:"billStatusCode"` // 01 Success, 04 Cancel, 05 Refund
	ResultYN                string         `json:"resultYN"`       // Y paid, N not yet paid
	ReferenceNo             FlexibleString `json:"referenceNo"`
	TransactionHash         string         `json:"transactionHash"`
}

// doPPCBankRequest is the shared HTTP logic for authenticated PPCBank
// calls. Handles the Bearer token and retries once on resultCode 900024
// ("Token has expired") by force-refreshing — same reasoning as the
// 401-retry logic in bakong_payment.go, just keyed off PPCBank's own
// in-body error code instead of an HTTP status.
func doPPCBankRequest(path string, payload any, out any) error {
	call := func(token string) (*http.Response, error) {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest(http.MethodPost, config.Get().PPCBankAPIBaseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "BubbleWhite-Backend/1.0")
		return httpClient.Do(req)
	}

	readHeader := func(resp *http.Response) ([]byte, ppcbankHeader, error) {
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, ppcbankHeader{}, fmt.Errorf("failed to read PPCBank response: %w", err)
		}
		var wrapper struct {
			Header ppcbankHeader `json:"header"`
		}
		if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
			return nil, ppcbankHeader{}, fmt.Errorf("failed to parse PPCBank response: %w", err)
		}
		return bodyBytes, wrapper.Header, nil
	}

	token, err := getPPCBankToken(false)
	if err != nil {
		return err
	}

	resp, err := call(token)
	if err != nil {
		log.Printf("ppcbank: %s request failed: %v", path, err)
		return fmt.Errorf("failed to reach PPCBank API: %w", err)
	}
	bodyBytes, header, err := readHeader(resp)
	if err != nil {
		return err
	}

	if !header.Result && header.ResultCode == "900024" {
		invalidatePPCBankToken()
		freshToken, err := getPPCBankToken(true)
		if err != nil {
			return err
		}
		resp2, err := call(freshToken)
		if err != nil {
			log.Printf("ppcbank: retry after token expiry also failed: %v", err)
			return fmt.Errorf("failed to reach PPCBank API: %w", err)
		}
		bodyBytes, header, err = readHeader(resp2)
		if err != nil {
			return err
		}
	}

	if !header.Result {
		log.Printf("ppcbank: %s failed, resultCode=%s message=%q", path, header.ResultCode, header.ResultMessage)
		return fmt.Errorf("ppcbank: %s", header.ResultMessage)
	}

	if out != nil {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return fmt.Errorf("failed to parse PPCBank response body: %w", err)
		}
	}
	return nil
}

// GeneratePPCBankKHQRPayment creates a hosted PPCBank payment page and
// returns the URL to redirect the customer to. billNumber should be the
// order's own reference (see models.Order.Reference()) — PPCBank calls
// this the "Track Order ID"; both Check KHQR Payment Status and the
// webhook payload key off it, so it must be unique and stable for this order.
func GeneratePPCBankKHQRPayment(billNumber string, amount float64, currencyCode string) (string, error) {
	merchantCode := config.Get().PPCBankMerchantCode
	if merchantCode == "" {
		return "", ErrPPCBankNotSet
	}

	// successURL/errorURL are built PER-REQUEST, not used as the static
	// configured URL directly — the return page needs to know WHICH
	// order to check when the customer comes back, and billNumber is the
	// only thing that identifies it. Appending it as a query param is the
	// simplest way to carry that through PPCBank's redirect.
	successURL := appendQueryParam(config.Get().PPCBankSuccessURL, "billNumber", billNumber)
	errorURL := appendQueryParam(config.Get().PPCBankErrorURL, "billNumber", billNumber)

	payload := map[string]any{
		"header": map[string]string{"languageCode": "01", "channelTypeCode": "03"},
		"body": map[string]any{
			"merchantCode": merchantCode,
			"billNumber":   billNumber,
			"amount":       amount,
			"currencyCode": currencyCode,
			// 5 minutes — matches the KHQR scan window already used for
			// Bakong, for a consistent customer-facing experience across
			// both payment methods. PPCBank's own example in their docs
			// used 60 seconds, which felt too short for a real checkout.
			"expiredTime": 300,
			"successURL":  successURL,
			"errorURL":    errorURL,
		},
	}

	var result struct {
		Header ppcbankHeader `json:"header"`
		Body   struct {
			PaymentURL string `json:"paymentURL"`
		} `json:"body"`
	}
	if err := doPPCBankRequest("/api/v1/PMS1011", payload, &result); err != nil {
		return "", err
	}
	if result.Body.PaymentURL == "" {
		return "", errors.New("ppcbank: no payment URL returned")
	}
	return result.Body.PaymentURL, nil
}

// appendQueryParam adds a query param to a URL without needing to worry
// about whether the URL already has other query params (?  vs &). Returns
// the base unchanged if it's empty, rather than producing a malformed
// "?billNumber=..." URL from a blank configured successURL/errorURL.
func appendQueryParam(base, key, value string) string {
	if base == "" {
		return ""
	}
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%s%s=%s", base, separator, key, url.QueryEscape(value))
}

// CheckPPCBankPaymentStatus looks up a payment by billNumber (our order
// reference) — the poll/verify equivalent of Bakong's
// CheckBakongPaymentByMD5. Used both as a fallback when the webhook
// doesn't arrive (PPCBank's own docs say it has no retry mechanism), and
// as the authoritative re-check whenever the webhook DOES arrive — the
// webhook payload itself is never trusted on its own, see the webhook
// handler in order_service.go.
func CheckPPCBankPaymentStatus(billNumber string) (bool, *PPCBankPaymentDetail, error) {
	merchantCode := config.Get().PPCBankMerchantCode
	if merchantCode == "" {
		return false, nil, ErrPPCBankNotSet
	}

	payload := map[string]any{
		"header": map[string]string{"languageCode": "01", "channelTypeCode": "03"},
		"body":   map[string]string{"merchantCode": merchantCode, "billNumber": billNumber},
	}

	var result struct {
		Header ppcbankHeader        `json:"header"`
		Body   PPCBankPaymentDetail `json:"body"`
	}
	if err := doPPCBankRequest("/api/v1/PMS1024", payload, &result); err != nil {
		return false, nil, err
	}

	paid := result.Body.ResultYN == "Y"
	return paid, &result.Body, nil
}
