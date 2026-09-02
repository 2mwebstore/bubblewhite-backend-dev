package models

import "time"

// Category mirrors the frontend's `categories` array. Image is a full public
// URL pointing at the R2 bucket (or CDN in front of it), set by the admin
// panel after uploading via POST /api/admin/uploads.
//
// ID is an auto-generated technical primary key — never set by the client.
// Slug remains the human-chosen natural key that products actually
// reference (Product.CategoryID stores the slug, not this ID), so this
// change doesn't ripple into product filtering/associations at all.
type Category struct {
	ID        uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"type:varchar(100);not null"`
	Slug      string `json:"slug" gorm:"type:varchar(100);uniqueIndex;not null"`
	Image     string `json:"image" gorm:"type:varchar(500)"`
	SortOrder int    `json:"sortOrder" gorm:"default:0;index"` // manual display order — lower shows first, admin-controlled (see banners for the same pattern)
	// IsActive controls visibility on the storefront — GET /api/categories
	// (public) only returns active ones; the admin list (GET
	// /api/admin/categories) returns all of them regardless, so staff can
	// still find and re-enable a disabled category. Same on/off pattern
	// already used for Banner.IsActive.
	IsActive  bool      `json:"isActive" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// ProductCount is populated by CategoryRepository.FindAllWithProductCount
	// — not a real column (gorm:"-" keeps AutoMigrate from creating one),
	// just how many products currently reference this category's slug.
	// Lets the storefront (footer, shop filters, home category strip) hide
	// categories that don't have any products yet instead of linking to an
	// empty results page.
	ProductCount int64 `json:"productCount" gorm:"-"`
}
