package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type UserRepository struct {
	*BaseRepository[models.User]
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{BaseRepository: NewBaseRepository[models.User](db)}
}

// FindByEmail loads a user (with its Role preloaded) by email, for login.
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.DB.Preload("Role").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByIDWithRole loads a user with its Role preloaded.
func (r *UserRepository) FindByIDWithRole(id uint) (*models.User, error) {
	var user models.User
	if err := r.DB.Preload("Role").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
