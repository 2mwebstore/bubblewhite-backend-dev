package models

import "time"

// Customer is a storefront shopper account — entirely separate from User
// (admin/staff accounts with roles/permissions). A Customer has no role,
// no permissions, just their own profile.
//
// Phone is the required, guaranteed identifier (Cambodia-based storefront —
// phone/Telegram is the primary contact channel for most shoppers). Email
// is optional, so it's a pointer: multiple customers can have no email
// (NULL) without violating the unique index — MySQL treats every NULL as
// distinct, unlike two empty strings which would collide.
//
// IsActive lets an admin disable a customer's login (e.g. abuse, a
// duplicate account, a support request) without deleting their account or
// order history — checked at login time in CustomerService.Login.
type Customer struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"type:varchar(150);not null"`
	Phone        string    `json:"phone" gorm:"type:varchar(50);uniqueIndex;not null"`
	Email        *string   `json:"email" gorm:"type:varchar(150);uniqueIndex"`
	PasswordHash string    `json:"-" gorm:"type:varchar(255);not null"`
	IsActive     bool      `json:"isActive" gorm:"default:true"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
