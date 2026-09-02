package services

import (
	"errors"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"

	"gorm.io/gorm"
)

type RoleService struct {
	Roles *repositories.RoleRepository
}

func NewRoleService(roles *repositories.RoleRepository) *RoleService {
	return &RoleService{Roles: roles}
}

// List returns every role, newest first. Role has no created_at column, so
// id DESC is used as the "latest" proxy — a higher auto-increment id was
// always created later.
func (s *RoleService) List() ([]models.Role, error) {
	return s.Roles.FindAll(func(db *gorm.DB) *gorm.DB {
		return db.Order("id DESC")
	})
}

func (s *RoleService) GetByID(id uint) (*models.Role, error) {
	return s.Roles.FindByID(id)
}

func (s *RoleService) GetBySlug(slug string) (*models.Role, error) {
	return s.Roles.FindBySlug(slug)
}

func (s *RoleService) Create(r *models.Role) error {
	return s.Roles.Create(r)
}

// Update replaces a role's name/description/permissions. System roles
// (IsSystem, e.g. "admin") can have their permissions edited but not be
// renamed away from their slug or deleted — see Delete below.
func (s *RoleService) Update(r *models.Role) error {
	return s.Roles.Update(r)
}

func (s *RoleService) Delete(id uint) error {
	role, err := s.Roles.FindByID(id)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return errors.New("this role is built-in and can't be deleted")
	}
	return s.Roles.Delete(id)
}
