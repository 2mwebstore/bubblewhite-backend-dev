package services

import (
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
)

type SettingsService struct {
	Settings *repositories.SettingsRepository
}

func NewSettingsService(settings *repositories.SettingsRepository) *SettingsService {
	return &SettingsService{Settings: settings}
}

func (s *SettingsService) Get() (*models.Settings, error) {
	return s.Settings.Get()
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
	current.CashPaymentEnabled = patch.CashPaymentEnabled
	current.BakongPaymentEnabled = patch.BakongPaymentEnabled
	current.PPCBankPaymentEnabled = patch.PPCBankPaymentEnabled
	current.Latitude = patch.Latitude
	current.Longitude = patch.Longitude
	current.DeliveryDistanceKm = patch.DeliveryDistanceKm

	if err := s.Settings.Update(current); err != nil {
		return nil, err
	}
	return current, nil
}
