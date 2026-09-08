package config

import (
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

// Config is the process-wide, read-only configuration snapshot. Load it
// once at boot via LoadConfig(), then read it anywhere with Get().
type Config struct {
	AppEnv      string
	Port        string
	CORSOrigins []string

	// IPs exempt from rate limiting entirely — e.g. your own office IP,
	// or a monitoring/health-check service. Comma-separated, optional;
	// empty means no exemptions.
	RateLimitAllowlist []string

	// Google/Facebook sign-in — see services/oauth_google.go and
	// oauth_facebook.go. GoogleClientID must match the "aud" claim on every
	// Google ID token this app accepts (see verifyGoogleIDToken) — without
	// it correctly set, EVERY Google sign-in attempt fails closed, which is
	// the safe default: a missing/misconfigured audience must never be
	// treated as "accept any token".
	GoogleClientID string
	// FacebookAppID/FacebookAppSecret are used together to call Facebook's
	// debug_token endpoint, which confirms an access token was actually
	// issued to THIS app (not a token a malicious client obtained from a
	// different Facebook app and is replaying here) — see
	// oauth_facebook.go's VerifyAccessToken.
	FacebookAppID     string
	FacebookAppSecret string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	JWTSecret       string
	JWTExpiresHours int

	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2Endpoint        string
	R2PublicURL       string

	SeedAdminEmail    string
	SeedAdminPassword string

	// PPCBank Payment Gateway — a KHQR-based payment option. Their own
	// docs are explicit that HTTP status is 200 even on logical failure (see
	// services/ppcbank_payment.go) — the actual result always lives in
	// the response body, never inferred from status alone.
	PPCBankAPIBaseURL   string
	PPCBankMerchantCode string
	PPCBankPassword     string
	// Where PPCBank redirects the customer after payment — must be a
	// full URL on THIS site (e.g. https://bubblewhite.co/orders/ppcbank-return).
	PPCBankSuccessURL string
	PPCBankErrorURL   string

	// Telegram bot notification — sends a message to a group chat whenever
	// an order's payment is confirmed. Like the other third-party API
	// credentials in this app, kept as env vars (not admin-editable
	// Settings fields) since a bot token is a real secret. Both must be
	// set for notifications to actually send; if either is empty,
	// notifications are silently skipped rather than erroring — this is
	// an enhancement, not something checkout should ever depend on.
	TelegramBotToken string
	TelegramChatID   string
	// TelegramBotUsername is public (needed to build the t.me/<username>
	// deep link returned to the frontend by CustomerController.RequestOTP)
	// — distinct from TelegramBotToken, which stays backend-only.
	TelegramBotUsername string

	// Plasgate — Cambodia's largest SMS gateway, used as the paid
	// fallback for OTP delivery when a customer has no Telegram linked
	// (see services/sms_plasgate.go and services/otp_service.go).
	// PrivateKey/SecretKey come from the Plasgate account dashboard;
	// SenderID is the "from" name recipients see (subject to Plasgate's
	// own sender-ID registration rules for Cambodia).
	PlasgatePrivateKey string
	PlasgateSecretKey  string
	PlasgateSenderID   string

	// FrontendBaseURL is the storefront's public origin (e.g.
	// "https://bubblewhite.co", no trailing slash) — used to build real,
	// clickable links in Telegram notifications (e.g. straight to
	// /admin/orders/{id} for reviewing a newly-paid order). Not required
	// for notifications to work at all; if empty, the order reference is
	// just shown as plain text instead of a link.
	FrontendBaseURL string
}

var (
	instance *Config
	once     sync.Once
)

// LoadConfig reads .env (if present) and environment variables into the
// singleton Config. Call this once from main(); subsequent calls are no-ops.
func LoadConfig() *Config {
	once.Do(func() {
		if err := godotenv.Load(); err != nil {
			log.Println("config: no .env file found, using system environment variables")
		}

		instance = &Config{
			AppEnv:      getEnv("APP_ENV", "development"),
			Port:        getEnv("PORT", "8080"),
			CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),

			RateLimitAllowlist: splitCSV(getEnv("RATE_LIMIT_ALLOWLIST", "")),

			GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
			FacebookAppID:     getEnv("FACEBOOK_APP_ID", ""),
			FacebookAppSecret: getEnv("FACEBOOK_APP_SECRET", ""),

			DBHost:     getEnv("DB_HOST", "127.0.0.1"),
			DBPort:     getEnv("DB_PORT", "3306"),
			DBUser:     getEnv("DB_USER", "root"),
			DBPassword: getEnv("DB_PASSWORD", ""),
			DBName:     getEnv("DB_NAME", "bubblewhite"),

			JWTSecret:       getEnv("JWT_SECRET", "insecure-dev-secret-change-me"),
			JWTExpiresHours: getEnvInt("JWT_EXPIRES_HOURS", 72),

			R2AccountID:       getEnv("R2_ACCOUNT_ID", ""),
			R2AccessKeyID:     getEnv("R2_ACCESS_KEY_ID", ""),
			R2SecretAccessKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
			R2Bucket:          getEnv("R2_BUCKET", ""),
			R2Endpoint:        getEnv("R2_ENDPOINT", ""),
			R2PublicURL:       getEnv("R2_PUBLIC_URL", ""),

			SeedAdminEmail:    getEnv("SEED_ADMIN_EMAIL", "admin@bubblewhite.co"),
			SeedAdminPassword: getEnv("SEED_ADMIN_PASSWORD", "ChangeMe123!"),

			// Default matches the sandbox domain shown in PPCBank's own
			// official spec document (PPCB_Payment_Gateway_API_v_1_0_4) —
			// NOT api.ppcbank.com, which was only ever seen as an example
			// in the web developer portal specifically for /security_check
			// and may be a different (possibly production) environment.
			// Override via env var once you know which is correct for
			// your actual merchant account.
			PPCBankAPIBaseURL:   getEnv("PPCBANK_API_BASE_URL", "https://paytest.ppcbank.com.kh"),
			PPCBankMerchantCode: getEnv("PPCBANK_MERCHANT_CODE", ""),
			PPCBankPassword:     getEnv("PPCBANK_PASSWORD", ""),
			PPCBankSuccessURL:   getEnv("PPCBANK_SUCCESS_URL", ""),
			PPCBankErrorURL:     getEnv("PPCBANK_ERROR_URL", ""),

			TelegramBotToken:    getEnv("TELEGRAM_BOT_TOKEN", ""),
			TelegramChatID:      getEnv("TELEGRAM_CHAT_ID", ""),
			TelegramBotUsername: getEnv("TELEGRAM_BOT_USERNAME", ""),

			PlasgatePrivateKey: getEnv("PLASGATE_PRIVATE_KEY", ""),
			PlasgateSecretKey:  getEnv("PLASGATE_SECRET_KEY", ""),
			PlasgateSenderID:   getEnv("PLASGATE_SENDER_ID", ""),
			FrontendBaseURL:    getEnv("FRONTEND_BASE_URL", ""),
		}
	})
	return instance
}

// Get returns the already-loaded config. Panics if LoadConfig hasn't run yet
// — that's intentional, it means main() forgot to call it.
func Get() *Config {
	if instance == nil {
		log.Fatal("config: Get() called before LoadConfig()")
	}
	return instance
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
