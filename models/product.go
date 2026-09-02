package models

import "time"

// Product mirrors the frontend's `products` array. Images is an ordered
// array of full public URLs (R2/CDN) — images[0] is the grid thumbnail, the
// full array powers the gallery + lightbox on the product detail page.
type Product struct {
	ID          string               `json:"id" gorm:"primaryKey;type:varchar(50)"`
	Name        string               `json:"name" gorm:"type:varchar(255);not null"`
	Price       float64              `json:"price" gorm:"type:decimal(10,2);not null"`
	CompareAt   *float64             `json:"compareAt" gorm:"type:decimal(10,2)"`
	CategoryID  string               `json:"category" gorm:"column:category_id;type:varchar(50);index;not null"`
	Badge       *string              `json:"badge" gorm:"type:varchar(50)"`
	Featured    bool                 `json:"featured" gorm:"default:false;index"`
	Sizes       JSONColumn[[]string] `json:"sizes" gorm:"type:json"`
	Images      JSONColumn[[]string] `json:"images" gorm:"type:json"`
	Description string               `json:"description" gorm:"type:text"`
	CreatedAt   time.Time            `json:"createdAt"`
	UpdatedAt   time.Time            `json:"updatedAt"`
}
