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

// ErrOAuthAccountNoPassword is returned when a phone/password login is
// attempted against an account that only ever signed up via Google or
// Facebook (PasswordHash is nil) — kept distinct from
// ErrInvalidCustomerCredentials so the frontend can show "this account
// uses Google/Facebook sign-in" instead of a generic wrong-password
// message. Confirming the identifier belongs to a real account is a very
// minor information leak, but the UX cost of NOT saying this (a customer
// endlessly retrying a password that was simply never set) is worse for a
// consumer storefront like this one.
var ErrOAuthAccountNoPassword = errors.New("this account signs in with google or facebook")

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

	customer := &models.Customer{Name: name, Phone: &phone, Email: emailPtr, PasswordHash: &hash, IsActive: true}
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
	if customer.PasswordHash == nil {
		return nil, ErrOAuthAccountNoPassword
	}
	if !utils.CheckPassword(*customer.PasswordHash, password) {
		return nil, ErrInvalidCustomerCredentials
	}
	if !customer.IsActive {
		return nil, ErrCustomerInactive
	}
	return customer, nil
}

// LoginOrRegisterWithGoogle implements Google's recommended "find or
// create" flow for verified sign-in, in three steps:
//  1. Already linked? A previous Google sign-in already set GoogleID on a
//     customer record — return it directly.
//  2. Not linked, but the verified email matches an existing customer
//     (someone who originally registered with phone+password) — link this
//     Google identity to that same account rather than creating a
//     duplicate, so their order history stays under one account regardless
//     of which method they use to sign in next time.
//  3. Neither — this is a genuinely new customer. Phone and PasswordHash
//     are left nil: Google never provides a phone number, and there's no
//     local password to set. Phone gets collected later, at checkout (see
//     OrderService.Checkout's ErrPhoneRequired), not forced at signup.
//
// email may be empty if Google didn't return a verified one — still
// creates/links correctly, just skips the email-matching step.
func (s *CustomerService) LoginOrRegisterWithGoogle(googleSub, email, name string) (*models.Customer, error) {
	if customer, err := s.Customers.FindByGoogleID(googleSub); err == nil {
		if !customer.IsActive {
			return nil, ErrCustomerInactive
		}
		return customer, nil
	}

	if email != "" {
		if existing, err := s.Customers.FindByEmail(email); err == nil {
			if !existing.IsActive {
				return nil, ErrCustomerInactive
			}
			existing.GoogleID = &googleSub
			if err := s.Customers.Update(existing); err != nil {
				return nil, err
			}
			return existing, nil
		}
	}

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	customer := &models.Customer{Name: name, Email: emailPtr, GoogleID: &googleSub, IsActive: true}
	if err := s.Customers.Create(customer); err != nil {
		return nil, err
	}
	return customer, nil
}

// LoginOrRegisterWithFacebook mirrors LoginOrRegisterWithGoogle exactly,
// just keyed on FacebookID instead of GoogleID — see that function's own
// comment for the full find-or-link-or-create reasoning. Facebook's email
// field is more often empty than Google's (a user can decline to share it,
// or have signed up to Facebook with a phone number instead of an email),
// so the email-matching step is skipped more often here in practice, not
// because the logic differs.
func (s *CustomerService) LoginOrRegisterWithFacebook(facebookID, email, name string) (*models.Customer, error) {
	if customer, err := s.Customers.FindByFacebookID(facebookID); err == nil {
		if !customer.IsActive {
			return nil, ErrCustomerInactive
		}
		return customer, nil
	}

	if email != "" {
		if existing, err := s.Customers.FindByEmail(email); err == nil {
			if !existing.IsActive {
				return nil, ErrCustomerInactive
			}
			existing.FacebookID = &facebookID
			if err := s.Customers.Update(existing); err != nil {
				return nil, err
			}
			return existing, nil
		}
	}

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	customer := &models.Customer{Name: name, Email: emailPtr, FacebookID: &facebookID, IsActive: true}
	if err := s.Customers.Create(customer); err != nil {
		return nil, err
	}
	return customer, nil
}

// LoginOrRegisterWithTelegram is simpler than the Google/Facebook
// equivalents: Telegram never provides an email, so there's no
// email-matching step to attempt — a customer either already has this
// exact TelegramID linked, or this is a genuinely new customer. Someone
// who registered with phone+password first and later signs in with
// Telegram will end up with two separate accounts unless they're
// manually linked (e.g. by an admin) — an inherent limitation of Telegram
// not sharing an email to match against, not something this service can
// work around.
func (s *CustomerService) LoginOrRegisterWithTelegram(telegramID, name string) (*models.Customer, error) {
	if customer, err := s.Customers.FindByTelegramID(telegramID); err == nil {
		if !customer.IsActive {
			return nil, ErrCustomerInactive
		}
		return customer, nil
	}

	customer := &models.Customer{Name: name, TelegramID: &telegramID, IsActive: true}
	if err := s.Customers.Create(customer); err != nil {
		return nil, err
	}
	return customer, nil
}

func (s *CustomerService) GetByID(id uint) (*models.Customer, error) {
	return s.Customers.FindByID(id)
}

// UpdateProfile updates name/phone/email — checks for a phone/email
// conflict against OTHER customers before saving (excluding the caller's
// own current value, so keeping your existing phone/email never falsely
// reads as a conflict), same pattern as Register. Both phone and email
// stay nullable — passing "" clears either back to nil rather than storing
// an empty string. Phone is optional here (unlike Register) specifically
// so a Google/Facebook customer without one yet can still save the rest of
// their profile — they're only ever required to provide it at checkout.
func (s *CustomerService) UpdateProfile(id uint, name, phone, email string) (*models.Customer, error) {
	customer, err := s.Customers.FindByID(id)
	if err != nil {
		return nil, err
	}

	currentPhone := ""
	if customer.Phone != nil {
		currentPhone = *customer.Phone
	}

	var phonePtr *string
	if phone != "" {
		if phone != currentPhone {
			if existing, err := s.Customers.FindByPhone(phone); err == nil && existing.ID != id {
				return nil, ErrPhoneAlreadyRegistered
			}
		}
		phonePtr = &phone
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
	customer.Phone = phonePtr
	customer.Email = emailPtr
	if err := s.Customers.Update(customer); err != nil {
		return nil, err
	}
	return customer, nil
}

// ChangePassword handles two cases: a normal password change (verifies
// currentPassword first, same as always), and a Google/Facebook-only
// account setting a LOCAL password for the first time (PasswordHash is
// nil, so there's nothing to verify against — currentPassword is ignored
// in that case rather than rejected, since requiring a password that was
// never set would make it impossible for these customers to ever add one).
func (s *CustomerService) ChangePassword(id uint, currentPassword, newPassword string) error {
	customer, err := s.Customers.FindByID(id)
	if err != nil {
		return err
	}
	if customer.PasswordHash != nil {
		if !utils.CheckPassword(*customer.PasswordHash, currentPassword) {
			return errors.New("current password is incorrect")
		}
	}
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	customer.PasswordHash = &hash
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
	customer.PasswordHash = &hash
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
