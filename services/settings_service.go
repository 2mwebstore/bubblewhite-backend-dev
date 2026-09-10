package services

import (
	"time"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

// settingsCacheKey: there's only ever one settings row, so a single
// fixed key is enough — no per-request variation to encode into it.
const settingsCacheKey = "settings"

// settingsCacheTTL is a safety net, not the primary invalidation
// mechanism — Update() below actively re-caches the fresh value the
// moment it's saved, so in normal operation a stale read would only
// ever happen if this process's cache and the database somehow
// disagreed outside of Update() itself (which doesn't happen today,
// since this is the only write path). The TTL just bounds how long
// that could ever last if it somehow did.
const settingsCacheTTL = 10 * time.Minute

type SettingsService struct {
	Settings *repositories.SettingsRepository
	Cache    *utils.Cache
}

func NewSettingsService(settings *repositories.SettingsRepository) *SettingsService {
	return &SettingsService{Settings: settings, Cache: utils.NewCache()}
}

// Get is called on every single storefront page load (header/footer
// company info, socials, logo) — exactly the kind of read-heavy, rarely-
// changing endpoint worth caching. Falls back to the database
// transparently on a cache miss; callers can't tell the difference
// except by response time.
func (s *SettingsService) Get() (*models.Settings, error) {
	if cached, ok := s.Cache.Get(settingsCacheKey); ok {
		return cached.(*models.Settings), nil
	}
	settings, err := s.Settings.Get()
	if err != nil {
		return nil, err
	}
	s.Cache.Set(settingsCacheKey, settings, settingsCacheTTL)
	return settings, nil
}

// Update replaces the settings row with the given values. This is a full
// PUT — the admin panel's Settings form always submits every field, so
// treating "" as "not provided" (as an older version of this did) silently
// ignored explicit clears, e.g. removing the logo would never actually
// persist since patch.LogoURL == "" looked identical to "field omitted".
func (s *SettingsService) Update(patch *models.Settings) (*models.Settings, error) {
	current, err := s.Settings.Get()
	if err != nil {
		return nil, err
	}

	current.CompanyName = patch.CompanyName
	current.CompanyDetail = patch.CompanyDetail
	current.FooterTagline = patch.FooterTagline
	current.ContactEmail = patch.ContactEmail
	current.ContactPhone = patch.ContactPhone
	current.ContactAddress = patch.ContactAddress
	current.WorkingHours = patch.WorkingHours
	current.FacebookURL = patch.FacebookURL
	current.InstagramURL = patch.InstagramURL
	current.TiktokURL = patch.TiktokURL
	current.TelegramURL = patch.TelegramURL
	current.LogoURL = patch.LogoURL
	// NOT current.CashPaymentEnabled / current.PPCBankPaymentEnabled —
	// these two fields are now dead weight: the PaymentMethod table (see
	// /admin/payment_method) replaced them as the actual source of truth
	// for checkout, and they're only ever read once, during the one-time
	// seed migration on a brand-new database. Leaving them editable here
	// would silently do nothing while looking like a working toggle —
	// worse than removing the fields outright, since the model still
	// needs them for that migration path.
	current.Latitude = patch.Latitude
	current.Longitude = patch.Longitude
	current.DeliveryDistanceKm = patch.DeliveryDistanceKm
	current.ShippingFee = patch.ShippingFee
	current.BackupTelegramGroupID = patch.BackupTelegramGroupID
	current.BackupTelegramBotToken = patch.BackupTelegramBotToken

	if err := s.Settings.Update(current); err != nil {
		return nil, err
	}
	// Re-cache the fresh value immediately rather than just invalidating
	// — an admin saving settings and then immediately viewing the
	// storefront should never see a stale cached response just because
	// the next read happened to land before a lazy repopulation would.
	s.Cache.Set(settingsCacheKey, current, settingsCacheTTL)
	return current, nil
}
