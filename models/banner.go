package models

import "time"

// Banner is one slide in the storefront home page's hero carousel, managed
// from the admin panel instead of being hard-coded in the frontend.
type Banner struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ImageURL  string    `json:"imageUrl" gorm:"type:varchar(500);not null"`
	Alt       string    `json:"alt" gorm:"type:varchar(255)"`
	LinkURL   string    `json:"linkUrl" gorm:"type:varchar(500)"` // optional — where clicking the slide goes
	SortOrder int       `json:"sortOrder" gorm:"default:0;index"`
	IsActive  bool      `json:"isActive" gorm:"default:true;index"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
