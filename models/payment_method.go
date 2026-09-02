package models

import "time"

// PaymentMethod is a CONFIGURATION/DISPLAY layer over the three payment
// integrations this app actually has real backend code for (cash, Bakong,
// PPCBank — see OrderService.Checkout / InitiatePPCBankCheckout). It does
// NOT let an admin add a genuinely new payment provider through this UI
// alone — each Code value here must correspond to an integration that
// already exists in code. What IS admin-configurable: display name, logo
// image, enabled/disabled, which one is the default pre-selected option
// at checkout, and display order.
type PaymentMethod struct {
	ID uint `json:"id" gorm:"primaryKey;autoIncrement"`
	// Code is the same internal identifier used everywhere else in the
	// codebase — models.PaymentMethodCash / PaymentMethodBakong /
	// PaymentMethodPPCBank. Never editable via the admin UI; it's the
	// link between this row and actual backend behavior.
	Code      string    `json:"code" gorm:"type:varchar(20);uniqueIndex;not null"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`
	ImageURL  string    `json:"imageUrl" gorm:"type:varchar(500)"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:false"`
	IsPrimary bool      `json:"isPrimary" gorm:"not null;default:false"`
	SortOrder int       `json:"sortOrder" gorm:"not null;default:0"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
