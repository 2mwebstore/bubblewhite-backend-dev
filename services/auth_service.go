package services

import (
	"errors"

	authdto "bubblewhite-backend/dto/auth"
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

type AuthService struct {
	Users *repositories.UserRepository
}

func NewAuthService(users *repositories.UserRepository) *AuthService {
	return &AuthService{Users: users}
}

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrAccountDisabled = errors.New("this account has been disabled")

// Login verifies credentials and returns a signed JWT plus a safe user summary.
func (s *AuthService) Login(req authdto.LoginRequest) (*authdto.LoginResponse, error) {
	user, err := s.Users.FindByEmail(req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	token, err := utils.SignToken(user.ID, user.Email, user.Role.Slug)
	if err != nil {
		return nil, err
	}

	return &authdto.LoginResponse{
		Token: token,
		User: authdto.UserSummary{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Role:  user.Role.Slug,
		},
	}, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *AuthService) ChangePassword(userID uint, currentPassword, newPassword string) error {
	user, err := s.Users.FindByIDWithRole(userID)
	if err != nil {
		return err
	}

	if !utils.CheckPassword(user.PasswordHash, currentPassword) {
		return errors.New("current password is incorrect")
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	return s.Users.Update(user)
}

// Me loads the current user (with role + permissions preloaded) — used by
// the frontend right after login (and on app boot) to know what to show.
func (s *AuthService) Me(userID uint) (*models.User, error) {
	return s.Users.FindByIDWithRole(userID)
}
