package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type ContactRepository struct {
	*BaseRepository[models.ContactMessage]
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{BaseRepository: NewBaseRepository[models.ContactMessage](db)}
}
