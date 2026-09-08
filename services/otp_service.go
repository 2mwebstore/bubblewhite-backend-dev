package services

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"bubblewhite-backend/config"
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OtpService struct {
	Otp           *repositories.OtpRepository
	TelegramLinks *repositories.TelegramPhoneLinkRepository
	SMS           *PlasgateSMSService
}

func NewOtpService(otp *repositories.OtpRepository, telegramLinks *repositories.TelegramPhoneLinkRepository, sms *PlasgateSMSService) *OtpService {
	return &OtpService{Otp: otp, TelegramLinks: telegramLinks, SMS: sms}
}

var ErrOtpExpiredOrNotFound = errors.New("verification code has expired or was not requested, please request a new one")
var ErrOtpTooManyAttempts = errors.New("too many incorrect attempts, please request a new code")
var ErrOtpIncorrect = errors.New("incorrect verification code")
var ErrTelegramLinkNotFound = errors.New("no pending telegram verification found")

const (
	otpLength      = 6
	otpValidFor    = 5 * time.Minute
	otpMaxAttempts = 5
)

// RequestResult tells the caller (the controller) what actually happened
// so it knows what to show the customer next — a link_token means "show
// the Open Telegram button", a channel of telegram_sent/sms means "show
// the code entry screen, a message already went out".
type RequestResult struct {
	Channel         string // "telegram_pending", "telegram_sent", or "sms"
	TelegramLinkURL string // only set when Channel == "telegram_pending"
}

// pendingRegistration bundles the rest of the registration form, already
// collected before OTP is ever sent — see RequestRegistrationOTP. nil for
// a plain login-via-OTP request (phone only, no account being created).
type pendingRegistration struct {
	Name         string
	Email        string
	PasswordHash string
}

// RequestOTP is the entry point for logging in via OTP — phone only, no
// account gets created off the back of this. Tries the free Telegram
// channel first via a known, previously-linked chat — only asks for the
// Telegram tap-through (or falls back to paid SMS, via
// RequestSMSFallback) when there's no link yet, since the whole point of
// TelegramPhoneLink is to make a returning customer's flow genuinely
// one-step.
func (s *OtpService) RequestOTP(phone, telegramBotUsername string) (*RequestResult, error) {
	return s.requestOTP(phone, telegramBotUsername, nil)
}

// RequestRegistrationOTP is the entry point when a customer submits the
// FULL registration form (name, phone, email, password) — collected up
// front, phone verification is the final step. passwordHash is already
// hashed by the caller (CustomerController), never the raw password, so
// it never needs to touch this service's own log lines or error paths in
// plaintext. See VerifyOTP for where this pending data actually turns
// into a real Customer account once the code is confirmed.
func (s *OtpService) RequestRegistrationOTP(name, phone, email, passwordHash, telegramBotUsername string) (*RequestResult, error) {
	return s.requestOTP(phone, telegramBotUsername, &pendingRegistration{Name: name, Email: email, PasswordHash: passwordHash})
}

func (s *OtpService) requestOTP(phone, telegramBotUsername string, pending *pendingRegistration) (*RequestResult, error) {
	if link, err := s.TelegramLinks.FindByPhone(phone); err == nil {
		code, err := generateOTP()
		if err != nil {
			return nil, err
		}
		if err := s.sendTelegramOTP(link.ChatID, code); err != nil {
			return nil, err
		}
		if err := s.saveOtpRequest(phone, code, "telegram_sent", nil, pending); err != nil {
			return nil, err
		}
		return &RequestResult{Channel: "telegram_sent"}, nil
	}

	// No existing link — generate a token for the deep-link tap-through
	// instead of a code. The code itself isn't generated yet: nobody can
	// read it until the customer actually opens Telegram, so generating
	// and "sending" one now would just be a code quietly expiring unused
	// while the customer is still looking at the button. The pending
	// registration fields ARE saved now though, on this same row —
	// TelegramBotController.Webhook updates this exact record in place
	// once the tap-through completes, so they carry through naturally.
	token, err := generateLinkToken()
	if err != nil {
		return nil, err
	}
	otp := &models.OtpRequest{
		Phone:     phone,
		Channel:   "telegram_pending",
		LinkToken: &token,
		ExpiresAt: time.Now().Add(otpValidFor),
	}
	applyPending(otp, pending)
	if err := s.Otp.Create(otp); err != nil {
		return nil, err
	}

	return &RequestResult{
		Channel:         "telegram_pending",
		TelegramLinkURL: fmt.Sprintf("https://t.me/%s?start=%s", telegramBotUsername, token),
	}, nil
}

// RequestSMSFallback is called when the customer explicitly chooses SMS
// instead of tapping through Telegram (no Telegram installed, or just a
// preference) — a separate, deliberate action rather than an automatic
// fallback, since SMS costs real money per message and shouldn't fire
// without the customer actually asking for it.
//
// Updates any existing pending request for this phone in place (rather
// than creating a fresh one) so pending registration fields already
// collected — the customer initially tried Telegram and is now falling
// back to SMS mid-registration — aren't lost. Falls back to creating a
// plain, pending-data-less request only if there's genuinely nothing to
// carry forward (a login-via-OTP attempt that never went through
// Telegram at all).
func (s *OtpService) RequestSMSFallback(phone string) error {
	code, err := generateOTP()
	if err != nil {
		return err
	}
	if err := s.SMS.SendOTP(phone, code); err != nil {
		return err
	}
	hash, err := utils.HashPassword(code)
	if err != nil {
		return err
	}

	if existing, err := s.Otp.FindLatestUnverifiedByPhone(phone); err == nil {
		existing.CodeHash = hash
		existing.Channel = "sms"
		existing.ExpiresAt = time.Now().Add(otpValidFor)
		return s.Otp.Update(existing)
	}

	otp := &models.OtpRequest{
		Phone:     phone,
		CodeHash:  hash,
		Channel:   "sms",
		ExpiresAt: time.Now().Add(otpValidFor),
	}
	return s.Otp.Create(otp)
}

// CompleteTelegramLink is called by the Telegram bot webhook when a
// customer taps through and their /start <token> message arrives. Only
// now does the actual code get generated — this is the point where it
// first becomes possible for anyone to read it. Updates the SAME
// OtpRequest row created back in requestOTP's telegram_pending branch, so
// any pending registration fields already on it are untouched and simply
// carry forward.
func (s *OtpService) CompleteTelegramLink(token, chatID string) error {
	pending, err := s.Otp.FindByLinkToken(token)
	if err != nil {
		return ErrTelegramLinkNotFound
	}
	if pending.Verified || time.Now().After(pending.ExpiresAt) {
		return ErrTelegramLinkNotFound
	}

	code, err := generateOTP()
	if err != nil {
		return err
	}
	if err := s.sendTelegramOTP(chatID, code); err != nil {
		return err
	}

	// Saved for every FUTURE request for this phone, not just this one —
	// this exact write is what turns the next login/registration attempt
	// for this phone into the one-step fast path instead of asking for
	// another tap-through.
	link := &models.TelegramPhoneLink{Phone: pending.Phone, ChatID: chatID}
	if err := s.TelegramLinks.Create(link); err != nil {
		return err
	}

	hash, err := utils.HashPassword(code)
	if err != nil {
		return err
	}
	pending.CodeHash = hash
	pending.Channel = "telegram_sent"
	pending.ExpiresAt = time.Now().Add(otpValidFor)
	return s.Otp.Update(pending)
}

// VerifyOTP checks a customer-submitted code against their most recent,
// still-valid request for that phone, and returns the full OtpRequest on
// success — the caller (CustomerController) reads its Pending* fields to
// decide what happens next: if they're set, this verification is what
// creates the actual Customer account (see CustomerService's own
// creation path off the back of this); if not, this was a plain
// login-via-OTP attempt for a phone that either does or doesn't already
// have an account.
//
// Locks out after otpMaxAttempts wrong tries — a 6-digit code has only a
// million possibilities, so without a hard attempt cap this would
// eventually be brute-forceable within its own 5-minute window. This
// entire check runs inside a transaction with a row-level lock
// (SELECT ... FOR UPDATE) rather than a plain read-then-write, because a
// plain read-then-write is a real, exploitable race: if an attacker fires
// many concurrent verify requests for the same phone, every one of them
// can read the SAME stale Attempts value before any of them writes back,
// so the counter never actually accumulates past ~1 no matter how many
// guesses run in parallel — silently defeating the entire attempt cap.
// The row lock forces concurrent attempts against the same phone to
// queue up and see each other's writes, so the count is always accurate
// regardless of how many requests arrive at once.
func (s *OtpService) VerifyOTP(phone, code string) (*models.OtpRequest, error) {
	var result *models.OtpRequest

	err := s.Otp.DB.Transaction(func(tx *gorm.DB) error {
		var otp models.OtpRequest
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("phone = ? AND verified = ? AND expires_at > NOW()", phone, false).
			Order("created_at DESC").
			First(&otp).Error
		if err != nil {
			return ErrOtpExpiredOrNotFound
		}
		if otp.CodeHash == "" {
			// Still telegram_pending — the customer hasn't tapped
			// through yet, so there's genuinely no code to check against.
			return ErrOtpExpiredOrNotFound
		}
		if otp.Attempts >= otpMaxAttempts {
			return ErrOtpTooManyAttempts
		}

		if !utils.CheckPassword(otp.CodeHash, code) {
			otp.Attempts++
			if err := tx.Save(&otp).Error; err != nil {
				return err
			}
			return ErrOtpIncorrect
		}

		otp.Verified = true
		if err := tx.Save(&otp).Error; err != nil {
			return err
		}
		result = &otp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OtpService) saveOtpRequest(phone, code, channel string, linkToken *string, pending *pendingRegistration) error {
	hash, err := utils.HashPassword(code)
	if err != nil {
		return err
	}
	otp := &models.OtpRequest{
		Phone:     phone,
		CodeHash:  hash,
		Channel:   channel,
		LinkToken: linkToken,
		ExpiresAt: time.Now().Add(otpValidFor),
	}
	applyPending(otp, pending)
	return s.Otp.Create(otp)
}

func applyPending(otp *models.OtpRequest, pending *pendingRegistration) {
	if pending == nil {
		return
	}
	otp.PendingName = &pending.Name
	otp.PendingPasswordHash = &pending.PasswordHash
	if pending.Email != "" {
		otp.PendingEmail = &pending.Email
	}
}

// sendTelegramOTP messages a specific customer chat directly — distinct
// from telegram_service.go's sendTelegramMessage, which is hardcoded to
// this business's own admin notification group and never returns an
// error (fine for a "nice to have" notification, wrong here: if this
// fails, the caller needs to know so it can surface an error rather than
// silently telling the customer a code was sent when it wasn't).
func (s *OtpService) sendTelegramOTP(chatID, code string) error {
	botToken := config.Get().TelegramBotToken
	if botToken == "" {
		return errors.New("telegram bot is not configured")
	}

	text := fmt.Sprintf("BubbleWhite\nលេខកូដផ្ទៀងផ្ទាត់របស់អ្នកគឺ: %s\n\nសូមកុំប្រាប់លេខកូដនេះទៅអ្នកដទៃ។", code)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload, err := json.Marshal(map[string]string{"chat_id": chatID, "text": text})
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("calling telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendMessage failed, status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

// generateOTP produces a cryptographically random N-digit numeric code —
// crypto/rand deliberately, never math/rand: math/rand is predictable
// enough (given its seed or enough samples) that an OTP built on it could
// theoretically be guessed, which defeats the entire point of the code
// being secret.
func generateOTP() (string, error) {
	digits := ""
	for i := 0; i < otpLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generating otp: %w", err)
		}
		digits += n.String()
	}
	return digits, nil
}

// generateLinkToken produces the random token embedded in a Telegram
// deep link (t.me/bot?start=<token>) — also crypto/rand, since this
// token is what proves a given Telegram chat is genuinely the one that
// clicked THIS specific link; a guessable token would let anyone claim
// someone else's pending verification.
func generateLinkToken() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("generating link token: %w", err)
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}
