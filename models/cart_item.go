package models

import "time"

// CartItem is one line in a customer's cart — backend-persisted (not
// localStorage), tied to their account, so it survives across devices and
// sessions. A product can appear more than once under different sizes AND
// now under different images too — (CustomerID, ProductID, Size, Image) is
// what identifies a unique line, so adding the same product/size but a
// DIFFERENT preview image (e.g. a different color shot) creates a separate
// line instead of merging quantities into an unrelated variant.
type CartItem struct {
	ID         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	CustomerID uint   `json:"customerId" gorm:"index;not null"`
	ProductID  string `json:"productId" gorm:"type:varchar(50);not null"`
	Size       string `json:"size" gorm:"type:varchar(20)"`
	// Image is the specific product image the customer was previewing when
	// they added it to the cart (product detail page's active thumbnail) —
	// not necessarily the product's first/default image. Empty when added
	// from a context with no preview concept (e.g. a grid quick-add
	// button), in which case CartService.List falls back to the product's
	// current first image.
	Image     string    `json:"image" gorm:"type:varchar(500)"`
	Quantity  int       `json:"quantity" gorm:"not null;default:1"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
