package models

import "time"

// ContactMessage stores a submission from the storefront's Contact page form.
type ContactMessage struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(150);not null"`
	Email     string    `json:"email" gorm:"type:varchar(150);not null"`
	Subject   string    `json:"subject" gorm:"type:varchar(255)"`
	Message   string    `json:"message" gorm:"type:text;not null"`
	IsRead    bool      `json:"isRead" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt"`
}
