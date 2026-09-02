package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"bubblewhite-backend/config"
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

// Package-level token cache — same reasoning as utils/jwt.go's direct
// config.Get() access, this is small stateless utility logic rather than a
// full repository-backed service. bakongDB is wired once at boot via
// SetBakongDB (see routes.Build()), same pattern as config.LoadConfig().
//
// Caching is two-tier: an in-memory value for the fast path (avoids a DB
// round-trip on every single check within the same process), backed by a
// database row so the token survives process restarts/redeploys instead
// of forcing a fresh renew_token call every time the service comes back
// up — Bakong's docs don't publish an exact token expiry, so this uses a
// conservative 12-hour TTL and force-refreshes on a 401.
var (
	bakongTokenMu   sync.Mutex
	bakongDB        *gorm.DB
	memToken        string
	memTokenAt      time.Time
	bakongTokenTTL  = 12 * time.Hour
	ErrBakongNotSet = errors.New("BAKONG_API_EMAIL is not configured")
	httpClient      = &http.Client{Timeout: 15 * time.Second}
)

// SetBakongDB wires the database connection this package uses to persist
// the Bakong API token — called once from routes.Build() at boot.
func SetBakongDB(db *gorm.DB) {
	bakongDB = db
}

func loadCachedBakongToken() (string, bool) {
	if memToken != "" && time.Since(memTokenAt) < bakongTokenTTL {
		return memToken, true
	}
	if bakongDB == nil {
		return "", false
	}
	var row models.BakongToken
	if err := bakongDB.First(&row).Error; err != nil {
		return "", false
	}
	if time.Since(row.FetchedAt) >= bakongTokenTTL {
		return "", false
	}
	// Warm the in-memory cache from the DB row so subsequent calls in this
	// same process skip the database round-trip entirely.
	memToken = row.Token
	memTokenAt = row.FetchedAt
	log.Println("bakong: reusing cached token from database (process restarted since it was last fetched)")
	return row.Token, true
}

func saveCachedBakongToken(token string) {
	memToken = token
	memTokenAt = time.Now()
	if bakongDB == nil {
		return
	}
	var row models.BakongToken
	if err := bakongDB.First(&row).Error; err != nil {
		bakongDB.Create(&models.BakongToken{Token: token, FetchedAt: memTokenAt})
		return
	}
	row.Token = token
	row.FetchedAt = memTokenAt
	bakongDB.Save(&row)
}

// GetCachedBakongAPIToken is the exported entry point for
// controllers/internal_controller.go — lets the Nuxt frontend fetch the
// SAME cached/DB-persisted token this backend already maintains, instead
// of independently calling Bakong's renew_token and keeping its own
// separate cache. Both services calling renew_token independently doubles
// up on Bakong's shared daily request quota for no real benefit — this is
// the fix, making this backend the single place that ever talks to
// Bakong's renew_token endpoint.
func GetCachedBakongAPIToken() (string, error) {
	return getBakongAPIToken(false)
}

func getBakongAPIToken(forceRefresh bool) (string, error) {
	bakongTokenMu.Lock()
	defer bakongTokenMu.Unlock()

	if !forceRefresh {
		if cached, ok := loadCachedBakongToken(); ok {
			return cached, nil
		}
	}

	// Optional manual override — if BAKONG_API_TOKEN is set, seed the cache
	// with it instead of immediately calling renew_token. Useful for
	// testing against a token issued outside this app, or as a stopgap if
	// BAKONG_API_EMAIL's registration has an issue. Only used to seed the
	// cache once per process — a manually-pasted token can't be "renewed"
	// the way an email-based one can, so once it expires or 401s,
	// subsequent refreshes fall through to the normal renew_token flow.
	if !forceRefresh {
		if manual := config.Get().BakongAPIToken; manual != "" {
			saveCachedBakongToken(manual)
			return manual, nil
		}
	}

	email := config.Get().BakongAPIEmail
	if email == "" {
		return "", ErrBakongNotSet
	}

	// Logged on every ACTUAL fetch (cache miss/expired/forced) — if this
	// line shows up on every single payment check instead of only every ~12
	// hours (or once per restart), that's the signal caching isn't working
	// as intended, rather than having to guess from the outside.
	log.Println("bakong: fetching a new API token via renew_token")

	body, _ := json.Marshal(map[string]string{"email": email})
	resp, err := httpClient.Post(config.Get().BakongAPIBaseURL+"/v1/renew_token", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("bakong: renew_token request failed: %v", err)
		return "", fmt.Errorf("failed to reach Bakong Open API: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		ResponseMessage string `json:"responseMessage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("bakong: failed to decode renew_token response: %v", err)
		return "", fmt.Errorf("failed to parse Bakong response: %w", err)
	}
	if result.Data.Token == "" {
		log.Printf("bakong: renew_token returned no token, message=%q", result.ResponseMessage)
		return "", fmt.Errorf("bakong: %s", result.ResponseMessage)
	}

	saveCachedBakongToken(result.Data.Token)
	log.Println("bakong: new token cached (in-memory + database)")
	return result.Data.Token, nil
}

func invalidateBakongAPIToken() {
	bakongTokenMu.Lock()
	defer bakongTokenMu.Unlock()
	memToken = ""
	if bakongDB != nil {
		bakongDB.Where("id > 0").Delete(&models.BakongToken{})
	}
}

// checkBakongTransaction is the shared HTTP logic behind every
// check_transaction_by_* verification method — same endpoint pattern
// (POST + Bearer + single-field JSON body), just a different field name
// and URL. Returns whether Bakong reports success (responseCode 0) plus
// whatever transaction detail it returned alongside that — the MD5/
// instructionRef checks only use the boolean, but check_transaction_by_hash
// returns much richer detail (tracking status, receiver bank) that the
// admin's transaction-detail lookup needs.
func checkBakongTransaction(endpoint, fieldName, fieldValue string) (bool, *BakongTransactionDetail, error) {
	doCheck := func(token string) (*http.Response, error) {
		body, _ := json.Marshal(map[string]string{fieldName: fieldValue})
		req, err := http.NewRequest(http.MethodPost, config.Get().BakongAPIBaseURL+endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		return httpClient.Do(req)
	}

	token, err := getBakongAPIToken(false)
	if err != nil {
		return false, nil, err
	}

	resp, err := doCheck(token)
	if err != nil {
		log.Printf("bakong: %s request failed: %v", endpoint, err)
		return false, nil, fmt.Errorf("failed to reach Bakong Open API: %w", err)
	}

	// A 401 means the cached token expired sooner than our TTL guess
	// assumed — refresh once and retry, rather than reporting a false
	// "not paid" for what's actually an auth problem.
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		invalidateBakongAPIToken()
		token, err = getBakongAPIToken(true)
		if err != nil {
			return false, nil, err
		}
		resp, err = doCheck(token)
		if err != nil {
			log.Printf("bakong: retry after 401 also failed: %v", err)
			return false, nil, fmt.Errorf("bakong authentication failed: %w", err)
		}
	}
	defer resp.Body.Close()

	// Read the body ONCE so it can be decoded twice: a minimal pass first
	// (just responseCode/responseMessage — the reliable pass/fail signal),
	// then a separate attempt at the richer detail. This split matters
	// because Bakong's API has been observed returning "amount" as either
	// a JSON number or a JSON string depending on context (their own docs
	// list it as type String, but sample responses show a raw number) —
	// a detail-decoding hiccup on a field like that must NEVER take down
	// the pass/fail check itself, which is what actually gates checkout.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("bakong: failed to read %s response body: %v", endpoint, err)
		return false, nil, fmt.Errorf("failed to read Bakong response: %w", err)
	}

	var minimal struct {
		ResponseCode    int    `json:"responseCode"`
		ResponseMessage string `json:"responseMessage"`
	}
	if err := json.Unmarshal(bodyBytes, &minimal); err != nil {
		log.Printf("bakong: failed to decode %s response: %v", endpoint, err)
		return false, nil, fmt.Errorf("failed to parse Bakong response: %w", err)
	}

	success := minimal.ResponseCode == 0

	var detail *BakongTransactionDetail
	if success {
		var full struct {
			Data *BakongTransactionDetail `json:"data"`
		}
		if err := json.Unmarshal(bodyBytes, &full); err != nil {
			// Best-effort — log it, but the pass/fail result above still
			// stands regardless of whether the richer detail parsed.
			log.Printf("bakong: %s succeeded but failed to decode transaction detail: %v", endpoint, err)
		} else {
			detail = full.Data
		}
	}

	// responseCode 0 = transaction found and completed successfully. Any
	// other value (NotFound, Failed, etc. per Bakong's docs) correctly
	// means "not paid" without needing to distinguish the exact reason
	// here — most callers only need a yes/no for gating checkout/admin
	// re-verification; GetBakongTransactionByHash below is the one that
	// actually surfaces detail to the caller.
	return success, detail, nil
}

// FlexibleNumber decodes from either a JSON number or a JSON string
// containing a number — Bakong's Open API has been observed returning
// fields like "amount" as both, depending on context, despite their own
// docs stating a fixed type. A strict float64 here would make the ENTIRE
// detail decode fail the moment one transaction happens to use the other
// representation.
type FlexibleNumber float64

func (f *FlexibleNumber) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = FlexibleNumber(v)
	return nil
}

// BakongTransactionDetail mirrors the `data` object Bakong's Open API
// returns on a successful transaction check. Every check_transaction_by_*
// endpoint returns at least the first block of fields; trackingStatus/
// receiverBank/receiverBankAccount are only populated by the full-hash and
// instruction-reference checks per Bakong's docs (not by the MD5 check).
type BakongTransactionDetail struct {
	Hash                string         `json:"hash"`
	FromAccountID       string         `json:"fromAccountId"`
	ToAccountID         string         `json:"toAccountId"`
	Currency            string         `json:"currency"`
	Amount              FlexibleNumber `json:"amount"`
	Description         string         `json:"description"`
	CreatedDateMs       FlexibleNumber `json:"createdDateMs"`
	AcknowledgedDateMs  FlexibleNumber `json:"acknowledgedDateMs"`
	TrackingStatus      string         `json:"trackingStatus"`
	ReceiverBank        string         `json:"receiverBank"`
	ReceiverBankAccount string         `json:"receiverBankAccount"`
}

// CheckBakongPaymentByMD5 verifies a transaction using the MD5 hash
// returned by bakong-khqr at QR generation time (see the Nuxt server's
// server/api/khqr.post.js). This is the reliable, PROVEN-working
// verification path — it's the same mechanism the customer-facing polling
// already uses successfully (server/api/khqr-status.post.js), so the
// backend's independent re-check uses the identical field rather than a
// second, differently-behaving one. The returned detail's Hash field is
// what OrderService stores as Order.TransactionHash, for the admin's
// richer check_transaction_by_hash lookup later (see
// GetBakongTransactionByHash) — the MD5 check itself doesn't return
// tracking status or receiver bank detail, only the full-hash check does.
func CheckBakongPaymentByMD5(md5 string) (bool, *BakongTransactionDetail, error) {
	if md5 == "" {
		return false, nil, errors.New("md5 is empty")
	}
	return checkBakongTransaction("/v1/check_transaction_by_md5", "md5", md5)
}

// CheckBakongPaymentByInstructionRef calls Bakong's Open API
// (check_transaction_by_instruction_ref). Kept available, but NOT the
// default verification path — instructionRef appears to be assigned by
// the PAYER's bank/app when they process their own transfer instruction,
// not something the merchant controls at KHQR-generation time, so a
// merchant-chosen billNumber used as instructionRef will generally fail
// to match. Use CheckBakongPaymentByMD5 unless you've independently
// confirmed how your integration actually populates instructionRef.
func CheckBakongPaymentByInstructionRef(instructionRef string) (bool, *BakongTransactionDetail, error) {
	if instructionRef == "" {
		return false, nil, errors.New("instruction reference is empty")
	}
	return checkBakongTransaction("/v1/check_transaction_by_instruction_ref", "instructionRef", instructionRef)
}

// GetBakongTransactionByHash fetches FULL transaction detail — including
// trackingStatus and receiverBank/receiverBankAccount, which the MD5 check
// does not return — using Bakong's check_transaction_by_hash endpoint.
// This is for the admin's detailed transaction lookup (e.g. confirming
// which bank the money actually landed in), not for checkout's pass/fail
// gating, which stays on CheckBakongPaymentByMD5.
func GetBakongTransactionByHash(hash string) (*BakongTransactionDetail, error) {
	if hash == "" {
		return nil, errors.New("no transaction hash recorded for this order yet — it may not have been paid via a verified Bakong check")
	}
	found, detail, err := checkBakongTransaction("/v1/check_transaction_by_hash", "hash", hash)
	if err != nil {
		return nil, err
	}
	if !found || detail == nil {
		return nil, errors.New("transaction not found")
	}
	return detail, nil
}
