package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type RoleRepository struct {
	*BaseRepository[models.Role]
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{BaseRepository: NewBaseRepository[models.Role](db)}
}

func (r *RoleRepository) FindBySlug(slug string) (*models.Role, error) {
	var role models.Role
	if err := r.DB.Where("slug = ?", slug).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}
