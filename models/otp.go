package models

import "time"

// OtpRequest is one phone-verification attempt — created either when a
// customer wants to log in via OTP (phone only) or when they submit the
// full registration form (name/email/password collected up front, phone
// verified as the final step — see OtpService.RequestRegistrationOTP).
// The verification step itself is what actually creates the Customer
// account when Pending* fields are present; see CustomerController.VerifyOTP.
//
// Channel tracks exactly where this attempt currently stands:
//   - "telegram_pending": no known TelegramPhoneLink for this phone yet.
//     CodeHash is empty — the code isn't generated until the customer
//     actually taps through to Telegram (see TelegramBotController.Webhook),
//     since generating and "sending" a code nobody can read yet would
//     just be a code that silently expires unused.
//   - "telegram_sent": sent via a Telegram chat — either an existing
//     TelegramPhoneLink (the fast path for a returning customer) or the
//     tap-through from telegram_pending completing.
//   - "sms": sent via Plasgate as the fallback, either because the
//     customer explicitly chose SMS or has no Telegram at all.
//
// CodeHash, never the raw code — same reasoning as PasswordHash on
// Customer: even a short-lived code shouldn't sit in the database in
// plaintext. Same reasoning applies to PendingPasswordHash below.
//
// Attempts guards against brute-forcing a short numeric code within its
// own expiry window — OtpService locks the request out after too many
// wrong tries, well before the expiry would.
type OtpRequest struct {
	ID        uint    `json:"id" gorm:"primaryKey;autoIncrement"`
	Phone     string  `json:"phone" gorm:"type:varchar(50);index;not null"`
	CodeHash  string  `json:"-" gorm:"type:varchar(255)"`
	Channel   string  `json:"channel" gorm:"type:varchar(20);not null"`
	LinkToken *string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	Attempts  int     `json:"-" gorm:"default:0"`
	Verified  bool    `json:"-" gorm:"default:false"`
	// PendingName/PendingEmail/PendingPasswordHash hold the rest of the
	// registration form, collected up front — nil for a plain
	// login-via-OTP request (phone only, no account being created), set
	// for a registration request. Once VerifyOTP confirms the code, these
	// are what CustomerService actually creates the account FROM,
	// exactly once, immediately — there's no separate second API call
	// where a customer manually re-submits this data after verifying.
	PendingName         *string   `json:"-" gorm:"type:varchar(150)"`
	PendingEmail        *string   `json:"-" gorm:"type:varchar(150)"`
	PendingPasswordHash *string   `json:"-" gorm:"type:varchar(255)"`
	ExpiresAt           time.Time `json:"-"`
	CreatedAt           time.Time `json:"createdAt"`
}

// TelegramPhoneLink is a durable, standalone record of "this phone number
// has proven it can receive messages at this Telegram chat" — kept
// separate from Customer (rather than a field on it) specifically because
// a phone can be verified via Telegram BEFORE any Customer account exists
// for it yet, e.g. mid-registration. Once this link exists, every future
// OTP request for this phone can be sent directly to ChatID — no more
// tap-through-Telegram step needed, which is what makes a returning
// customer's flow genuinely one-step (see OtpService.RequestOTP).
type TelegramPhoneLink struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Phone     string    `json:"phone" gorm:"type:varchar(50);uniqueIndex;not null"`
	ChatID    string    `json:"-" gorm:"type:varchar(64);not null"`
	CreatedAt time.Time `json:"createdAt"`
}
