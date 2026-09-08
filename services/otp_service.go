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

// RequestOTP is the main entry point when a customer submits a phone
// number to verify. Tries the free Telegram channel first via a known,
// previously-linked chat — only asks for the Telegram tap-through (or
// falls back to paid SMS, via RequestSMSFallback) when there's no link
// yet, since the whole point of TelegramPhoneLink is to make a returning
// customer's flow genuinely one-step.
func (s *OtpService) RequestOTP(phone, telegramBotUsername string) (*RequestResult, error) {
	if link, err := s.TelegramLinks.FindByPhone(phone); err == nil {
		code, err := generateOTP()
		if err != nil {
			return nil, err
		}
		if err := s.sendTelegramOTP(link.ChatID, code); err != nil {
			return nil, err
		}
		if err := s.saveOtpRequest(phone, code, "telegram_sent", nil); err != nil {
			return nil, err
		}
		return &RequestResult{Channel: "telegram_sent"}, nil
	}

	// No existing link — generate a token for the deep-link tap-through
	// instead of a code. The code itself isn't generated yet: nobody can
	// read it until the customer actually opens Telegram, so generating
	// and "sending" one now would just be a code quietly expiring unused
	// while the customer is still looking at the button.
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
// preference) — a separate, deliberate action from RequestOTP rather than
// an automatic fallback, since SMS costs real money per message and
// shouldn't fire without the customer actually asking for it.
func (s *OtpService) RequestSMSFallback(phone string) error {
	code, err := generateOTP()
	if err != nil {
		return err
	}
	if err := s.SMS.SendOTP(phone, code); err != nil {
		return err
	}
	return s.saveOtpRequest(phone, code, "sms", nil)
}

// CompleteTelegramLink is called by the Telegram bot webhook when a
// customer taps through and their /start <token> message arrives. Only
// now does the actual code get generated — this is the point where it
// first becomes possible for anyone to read it.
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
// still-valid request for that phone. Locks out after otpMaxAttempts
// wrong tries — a 6-digit code has only a million possibilities, so
// without a hard attempt cap this would eventually be brute-forceable
// within its own 5-minute window.
//
// On success, returns a VerificationToken the caller must present to
// actually complete a login or registration — see ConsumeVerificationToken
// and the model's own doc comment for why proving the code was correct
// isn't, by itself, enough to let a later, unrelated request claim this
// phone was verified by them.
func (s *OtpService) VerifyOTP(phone, code string) (string, error) {
	otp, err := s.Otp.FindLatestUnverifiedByPhone(phone)
	if err != nil {
		return "", ErrOtpExpiredOrNotFound
	}
	if otp.CodeHash == "" {
		// Still telegram_pending — the customer hasn't tapped through
		// yet, so there's genuinely no code to check against.
		return "", ErrOtpExpiredOrNotFound
	}
	if otp.Attempts >= otpMaxAttempts {
		return "", ErrOtpTooManyAttempts
	}

	if !utils.CheckPassword(otp.CodeHash, code) {
		otp.Attempts++
		_ = s.Otp.Update(otp)
		return "", ErrOtpIncorrect
	}

	token, err := generateLinkToken() // same shape/entropy requirement, reused rather than a near-duplicate generator
	if err != nil {
		return "", err
	}
	otp.Verified = true
	otp.VerificationToken = &token
	if err := s.Otp.Update(otp); err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeVerificationToken checks that a verification token is real,
// belongs to the exact phone number the caller is claiming, and hasn't
// already been used — then invalidates it so it can't be replayed for a
// second login/registration. Called from CustomerController right before
// actually logging in or creating an account off the back of an OTP flow.
func (s *OtpService) ConsumeVerificationToken(token, phone string) error {
	otp, err := s.Otp.FindByVerificationToken(token)
	if err != nil {
		return ErrOtpExpiredOrNotFound
	}
	if otp.Phone != phone {
		return ErrOtpExpiredOrNotFound
	}
	otp.VerificationToken = nil // single-use — cleared immediately so a retry/replay of this exact token fails
	return s.Otp.Update(otp)
}

func (s *OtpService) saveOtpRequest(phone, code, channel string, linkToken *string) error {
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
	return s.Otp.Create(otp)
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
