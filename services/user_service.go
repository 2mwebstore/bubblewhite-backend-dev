package services

import (
	"errors"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

// AdminRoleSlug is the built-in Administrator role's slug — used to protect
// admin accounts from being deleted or demoted via the Users list.
const AdminRoleSlug = "admin"

type UserService struct {
	Users *repositories.UserRepository
}

func NewUserService(users *repositories.UserRepository) *UserService {
	return &UserService{Users: users}
}

// List returns every staff account, newest first.
func (s *UserService) List() ([]models.User, error) {
	return s.Users.FindAll(func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	})
}

func (s *UserService) GetByID(id uint) (*models.User, error) {
	return s.Users.FindByIDWithRole(id)
}

// Create hashes the given plaintext password and creates a new staff account.
func (s *UserService) Create(u *models.User, plainPassword string) error {
	hash, err := utils.HashPassword(plainPassword)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	return s.Users.Create(u)
}

func (s *UserService) Update(u *models.User) error {
	return s.Users.Update(u)
}

// Delete removes a staff account — refuses to delete an Administrator, so
// the Users list can never be used to accidentally lock everyone out.
func (s *UserService) Delete(id uint) error {
	u, err := s.Users.FindByIDWithRole(id)
	if err != nil {
		return err
	}
	if u.Role.Slug == AdminRoleSlug {
		return errors.New("cannot delete an Administrator account")
	}
	return s.Users.Delete(id)
}

// SetActive enables/disables a staff account without deleting it.
func (s *UserService) SetActive(id uint, active bool) error {
	u, err := s.Users.FindByIDWithRole(id)
	if err != nil {
		return err
	}
	u.IsActive = active
	return s.Users.Update(u)
}

// AssignRole changes which role a user has — this is how permissions get
// assigned to a user in practice: pick a role, assign it. Refuses to move
// an Administrator to a different role — demoting the wrong admin from a
// dropdown is exactly the kind of mistake this exists to prevent. (Moving
// someone ELSE *into* the Administrator role is still allowed.)
func (s *UserService) AssignRole(id uint, roleID uint) error {
	u, err := s.Users.FindByIDWithRole(id)
	if err != nil {
		return err
	}
	if u.Role.Slug == AdminRoleSlug && u.RoleID != roleID {
		return errors.New("cannot change an Administrator's role")
	}
	u.RoleID = roleID
	return s.Users.Update(u)
}

// ResetPassword lets an admin set a new password for another user directly —
// no need to know their current password, unlike ChangePassword (self-service).
func (s *UserService) ResetPassword(id uint, newPassword string) error {
	u, err := s.Users.FindByIDWithRole(id)
	if err != nil {
		return err
	}
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	return s.Users.Update(u)
}
