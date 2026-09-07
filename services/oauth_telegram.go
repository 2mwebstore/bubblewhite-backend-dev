package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TelegramUser is the verified identity extracted from a Telegram Login
// Widget payload — only what CustomerService actually needs. Telegram
// never provides an email, unlike Google/Facebook — this is the one
// provider where LoginOrRegisterWithTelegram can never link an existing
// phone/password account by email, only by a previous TelegramID match.
type TelegramUser struct {
	ID   string
	Name string
}

type TelegramOAuthService struct {
	botToken string
}

func NewTelegramOAuthService(botToken string) *TelegramOAuthService {
	return &TelegramOAuthService{botToken: botToken}
}

var ErrTelegramNotConfigured = errors.New("telegram sign-in is not configured")
var ErrInvalidTelegramAuth = errors.New("invalid telegram login data")

// maxAuthAge rejects a payload whose auth_date is older than this — per
// Telegram's own docs, auth_date exists specifically so a captured/replayed
// payload can't be reused indefinitely. Every field in the payload
// (including auth_date itself) is otherwise attacker-controllable; only
// the hash proves it's genuinely from Telegram, so this check only means
// anything once the hash below has already verified.
const maxAuthAge = 5 * time.Minute

// TelegramAuthPayload mirrors exactly what the Telegram Login Widget calls
// its onauth JS callback with — see core.telegram.org/widgets/login. Every
// field except hash itself feeds into the signature check.
type TelegramAuthPayload struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

// Verify checks the payload's hash against Telegram's own signing
// algorithm and rejects anything stale, then returns the verified
// identity. Never trust this payload's fields directly — anyone can POST
// an arbitrary {id, first_name, ...} JSON body claiming to be any Telegram
// user; only a hash that correctly verifies against this bot's own token
// (which only Telegram's servers and this backend ever know) proves the
// payload is genuine.
func (t *TelegramOAuthService) Verify(payload TelegramAuthPayload) (*TelegramUser, error) {
	if t.botToken == "" {
		return nil, ErrTelegramNotConfigured
	}
	if payload.ID == 0 || payload.Hash == "" {
		return nil, ErrInvalidTelegramAuth
	}

	if time.Since(time.Unix(payload.AuthDate, 0)) > maxAuthAge {
		return nil, fmt.Errorf("%w: auth data has expired, please try signing in again", ErrInvalidTelegramAuth)
	}

	if !t.hashIsValid(payload) {
		return nil, ErrInvalidTelegramAuth
	}

	name := strings.TrimSpace(payload.FirstName + " " + payload.LastName)
	if name == "" {
		name = payload.Username
	}
	if name == "" {
		name = "Telegram User"
	}

	return &TelegramUser{ID: strconv.FormatInt(payload.ID, 10), Name: name}, nil
}

// hashIsValid implements Telegram's exact documented algorithm:
//  1. Build a "data-check-string" from every field except hash, as
//     key=value pairs, sorted alphabetically by key, joined with \n.
//     Fields the widget didn't actually send (empty last_name/username/
//     photo_url are common — not everyone has a username or a photo) are
//     excluded entirely, not included as key=.
//  2. secret_key = SHA256(bot_token) — the raw 32 bytes, not a hex string.
//  3. Compute HMAC-SHA256(data_check_string, secret_key), hex-encode it,
//     and compare to the received hash.
func (t *TelegramOAuthService) hashIsValid(payload TelegramAuthPayload) bool {
	fields := map[string]string{
		"id":         strconv.FormatInt(payload.ID, 10),
		"first_name": payload.FirstName,
		"last_name":  payload.LastName,
		"username":   payload.Username,
		"photo_url":  payload.PhotoURL,
		"auth_date":  strconv.FormatInt(payload.AuthDate, 10),
	}

	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+fields[k])
	}
	dataCheckString := strings.Join(lines, "\n")

	secretKey := sha256.Sum256([]byte(t.botToken))

	mac := hmac.New(sha256.New, secretKey[:])
	mac.Write([]byte(dataCheckString))
	computedHash := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison — a plain == here would let an attacker
	// use response-timing differences to guess the correct hash one byte
	// at a time. Exactly the same reasoning as comparing password hashes.
	return hmac.Equal([]byte(computedHash), []byte(payload.Hash))
}
