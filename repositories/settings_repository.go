package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type SettingsRepository struct {
	DB *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{DB: db}
}

// Get loads the single settings row, creating it with empty defaults if it
// somehow doesn't exist yet (normally seed.Run() creates it on first boot).
func (r *SettingsRepository) Get() (*models.Settings, error) {
	var s models.Settings
	err := r.DB.FirstOrCreate(&s, models.Settings{ID: models.SettingsID}).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SettingsRepository) Update(s *models.Settings) error {
	s.ID = models.SettingsID
	return r.DB.Save(s).Error
}
