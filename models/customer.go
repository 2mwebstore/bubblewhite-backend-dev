package models

import "time"

// Customer is a storefront shopper account — entirely separate from User
// (admin/staff accounts with roles/permissions). A Customer has no role,
// no permissions, just their own profile.
//
// Phone was originally the required, guaranteed identifier (Cambodia-based
// storefront — phone/Telegram is the primary contact channel for most
// shoppers). It's now nullable to support Google/Facebook sign-in, which
// never provides a phone number — those customers are asked for one at
// checkout instead (see OrderService.Checkout's ErrPhoneRequired), not at
// signup. Like Email, it's a pointer so multiple phone-less customers
// don't collide on the unique index — MySQL treats every NULL as distinct,
// unlike two empty strings which would collide.
//
// PasswordHash is nullable for the same reason: a customer who only ever
// signs in via Google/Facebook has no local password at all. CustomerService
// checks for this before ever attempting a password comparison.
//
// GoogleID/FacebookID/TelegramID link a customer record to their identity
// with that provider — nullable/unique, same pattern as Phone/Email. If a
// customer registered with phone+password first and later signs in with
// Google using the same email, CustomerService links the existing record
// rather than creating a duplicate (see LoginOrRegisterWithGoogle).
// Telegram never provides an email at all, so TelegramID can only ever be
// matched against itself, never linked by email the way Google/Facebook
// can (see LoginOrRegisterWithTelegram).
//
// IsActive lets an admin disable a customer's login (e.g. abuse, a
// duplicate account, a support request) without deleting their account or
// order history — checked at login time in CustomerService.Login.
type Customer struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"type:varchar(150);not null"`
	Phone        *string   `json:"phone" gorm:"type:varchar(50);uniqueIndex"`
	Email        *string   `json:"email" gorm:"type:varchar(150);uniqueIndex"`
	PasswordHash *string   `json:"-" gorm:"type:varchar(255)"`
	GoogleID     *string   `json:"-" gorm:"type:varchar(255);uniqueIndex"`
	FacebookID   *string   `json:"-" gorm:"type:varchar(255);uniqueIndex"`
	TelegramID   *string   `json:"-" gorm:"type:varchar(255);uniqueIndex"`
	IsActive     bool      `json:"isActive" gorm:"default:true"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
