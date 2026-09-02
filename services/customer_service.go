package services

import (
	"errors"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

type CustomerService struct {
	Customers *repositories.CustomerRepository
}

func NewCustomerService(customers *repositories.CustomerRepository) *CustomerService {
	return &CustomerService{Customers: customers}
}

var ErrEmailAlreadyRegistered = errors.New("email is already registered")
var ErrPhoneAlreadyRegistered = errors.New("phone number is already registered")
var ErrInvalidCustomerCredentials = errors.New("invalid phone/email or password")
var ErrCustomerInactive = errors.New("this account has been disabled")

// Register creates a new customer account. Phone is required and must be
// unique; email is optional (nil when not given, so multiple customers can
// have no email without colliding on the unique index — see Customer's doc
// comment). Both existing-value checks happen up front (rather than only
// relying on the DB's unique constraints) so the caller gets a clean,
// friendly field error instead of a raw duplicate-key database error —
// same pattern used for products/categories.
func (s *CustomerService) Register(name, phone, email, password string) (*models.Customer, error) {
	if _, err := s.Customers.FindByPhone(phone); err == nil {
		return nil, ErrPhoneAlreadyRegistered
	}

	var emailPtr *string
	if email != "" {
		if _, err := s.Customers.FindByEmail(email); err == nil {
			return nil, ErrEmailAlreadyRegistered
		}
		emailPtr = &email
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	customer := &models.Customer{Name: name, Phone: phone, Email: emailPtr, PasswordHash: hash, IsActive: true}
	if err := s.Customers.Create(customer); err != nil {
		return nil, err
	}
	return customer, nil
}

// Login accepts EITHER a phone number or an email as the identifier, since
// email is optional at registration — some customers only ever have a
// phone number to log in with. Rejects a disabled account with a distinct
// error so the frontend can show "your account has been disabled" instead
// of a generic "wrong password" message.
func (s *CustomerService) Login(identifier, password string) (*models.Customer, error) {
	customer, err := s.Customers.FindByPhone(identifier)
	if err != nil {
		customer, err = s.Customers.FindByEmail(identifier)
		if err != nil {
			return nil, ErrInvalidCustomerCredentials
		}
	}
	if !utils.CheckPassword(customer.PasswordHash, password) {
		return nil, ErrInvalidCustomerCredentials
	}
	if !customer.IsActive {
		return nil, ErrCustomerInactive
	}
	return customer, nil
}

func (s *CustomerService) GetByID(id uint) (*models.Customer, error) {
	return s.Customers.FindByID(id)
}

// UpdateProfile updates name/phone/email — checks for a phone/email
// conflict against OTHER customers before saving (excluding the caller's
// own current value, so keeping your existing phone/email never falsely
// reads as a conflict), same pattern as Register. Email stays nullable —
// passing "" clears it back to nil rather than storing an empty string.
func (s *CustomerService) UpdateProfile(id uint, name, phone, email string) (*models.Customer, error) {
	customer, err := s.Customers.FindByID(id)
	if err != nil {
		return nil, err
	}

	if phone != customer.Phone {
		if existing, err := s.Customers.FindByPhone(phone); err == nil && existing.ID != id {
			return nil, ErrPhoneAlreadyRegistered
		}
	}

	var emailPtr *string
	if email != "" {
		currentEmail := ""
		if customer.Email != nil {
			currentEmail = *customer.Email
		}
		if email != currentEmail {
			if existing, err := s.Customers.FindByEmail(email); err == nil && existing.ID != id {
				return nil, ErrEmailAlreadyRegistered
			}
		}
		emailPtr = &email
	}

	customer.Name = name
	customer.Phone = phone
	customer.Email = emailPtr
	if err := s.Customers.Update(customer); err != nil {
		return nil, err
	}
	return customer, nil
}

func (s *CustomerService) ChangePassword(id uint, currentPassword, newPassword string) error {
	customer, err := s.Customers.FindByID(id)
	if err != nil {
		return err
	}
	if !utils.CheckPassword(customer.PasswordHash, currentPassword) {
		return errors.New("current password is incorrect")
	}
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	customer.PasswordHash = hash
	return s.Customers.Update(customer)
}

// --- Admin management ---

// ListAllAdmin returns every customer, newest first, paginated — for the
// admin panel's customer list (matches the same pagination pattern used
// for products/users).
func (s *CustomerService) ListAllAdmin(page utils.PageParams) ([]models.Customer, int64, error) {
	total, err := s.Customers.Count()
	if err != nil {
		return nil, 0, err
	}
	customers, err := s.Customers.FindAll(
		func(db *gorm.DB) *gorm.DB { return db.Order("created_at DESC") },
		page.Scope(),
	)
	if err != nil {
		return nil, 0, err
	}
	return customers, total, nil
}

// AdminResetPassword lets an admin set a new password for a customer
// directly — no current password needed, same pattern as resetting a staff
// user's password.
func (s *CustomerService) AdminResetPassword(id uint, newPassword string) error {
	customer, err := s.Customers.FindByID(id)
	if err != nil {
		return err
	}
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	customer.PasswordHash = hash
	return s.Customers.Update(customer)
}

// SetActive enables/disables a customer's ability to log in, without
// deleting their account or order history.
func (s *CustomerService) SetActive(id uint, active bool) error {
	customer, err := s.Customers.FindByID(id)
	if err != nil {
		return err
	}
	customer.IsActive = active
	return s.Customers.Update(customer)
}
