package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type OtpRepository struct {
	*BaseRepository[models.OtpRequest]
}

func NewOtpRepository(db *gorm.DB) *OtpRepository {
	return &OtpRepository{BaseRepository: NewBaseRepository[models.OtpRequest](db)}
}

// FindByLinkToken looks up the pending OtpRequest a Telegram /start
// deep-link token belongs to — this is how TelegramController.Start
// matches an incoming Telegram message back to the specific verification
// attempt that generated the link, without the bot ever needing to know
// anything about phone numbers directly.
func (r *OtpRepository) FindByLinkToken(token string) (*models.OtpRequest, error) {
	var otp models.OtpRequest
	if err := r.DB.Where("link_token = ?", token).First(&otp).Error; err != nil {
		return nil, err
	}
	return &otp, nil
}

// FindLatestUnverifiedByPhone returns the most recent, still-valid
// verification attempt for a phone — what OtpService.VerifyOTP checks the
// customer's submitted code against. Excludes already-verified requests
// (a stale success shouldn't be re-checked) and expired ones (an old
// request from an earlier attempt shouldn't linger as valid just because
// nothing deleted it yet).
func (r *OtpRepository) FindLatestUnverifiedByPhone(phone string) (*models.OtpRequest, error) {
	var otp models.OtpRequest
	err := r.DB.Where("phone = ? AND verified = ? AND expires_at > NOW()", phone, false).
		Order("created_at DESC").
		First(&otp).Error
	if err != nil {
		return nil, err
	}
	return &otp, nil
}

// FindByVerificationToken looks up an already-verified OtpRequest by its
// post-verification proof token — what CustomerController's
// OTP-login/OTP-register handlers check before trusting that a phone
// number really was just verified by whoever is calling right now (see
// VerificationToken's own doc comment on the model for why this can't
// just be inferred from the phone number alone).
func (r *OtpRepository) FindByVerificationToken(token string) (*models.OtpRequest, error) {
	var otp models.OtpRequest
	if err := r.DB.Where("verification_token = ?", token).First(&otp).Error; err != nil {
		return nil, err
	}
	return &otp, nil
}

type TelegramPhoneLinkRepository struct {
	*BaseRepository[models.TelegramPhoneLink]
}

func NewTelegramPhoneLinkRepository(db *gorm.DB) *TelegramPhoneLinkRepository {
	return &TelegramPhoneLinkRepository{BaseRepository: NewBaseRepository[models.TelegramPhoneLink](db)}
}

// FindByPhone is the fast-path check OtpService makes before ever
// generating a link_token — if this phone already proved it can receive
// messages at a known Telegram chat, skip straight to sending, no
// tap-through needed.
func (r *TelegramPhoneLinkRepository) FindByPhone(phone string) (*models.TelegramPhoneLink, error) {
	var link models.TelegramPhoneLink
	if err := r.DB.Where("phone = ?", phone).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}
