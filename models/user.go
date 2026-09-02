package models

import "time"

// User is an admin-panel account. Regular shoppers don't get accounts —
// the storefront is catalog + enquiry only — so every User here is staff.
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"type:varchar(150);not null"`
	Email        string    `json:"email" gorm:"type:varchar(150);uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"type:varchar(255);not null"`
	RoleID       uint      `json:"roleId" gorm:"not null;index"`
	Role         Role      `json:"role" gorm:"foreignKey:RoleID"`
	IsActive     bool      `json:"isActive" gorm:"default:true"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
