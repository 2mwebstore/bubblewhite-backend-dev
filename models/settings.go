package models

import "time"

// Settings is a single-row table (always ID = 1) holding the company/contact
// info the storefront displays — footer, contact page, SEO — all editable
// from the admin panel instead of being hard-coded in the frontend.
type Settings struct {
	ID uint `json:"id" gorm:"primaryKey"`

	CompanyName   string `json:"companyName" gorm:"type:varchar(150)"`
	CompanyDetail string `json:"companyDetail" gorm:"type:text"`         // shown on the About page
	FooterTagline string `json:"footerTagline" gorm:"type:varchar(255)"` // short line shown under the logo in the site footer

	ContactEmail   string `json:"contactEmail" gorm:"type:varchar(150)"`
	ContactPhone   string `json:"contactPhone" gorm:"type:varchar(50)"`
	ContactAddress string `json:"contactAddress" gorm:"type:varchar(255)"`
	WorkingHours   string `json:"workingHours" gorm:"type:varchar(150)"`

	FacebookURL  string `json:"facebookUrl" gorm:"type:varchar(255)"`
	InstagramURL string `json:"instagramUrl" gorm:"type:varchar(255)"`
	TiktokURL    string `json:"tiktokUrl" gorm:"type:varchar(255)"`
	TelegramURL  string `json:"telegramUrl" gorm:"type:varchar(255)"`

	LogoURL string `json:"logoUrl" gorm:"type:varchar(500)"`

	// Legacy — no longer the source of truth for checkout. These were the
	// original payment-method toggles before the PaymentMethod table (see
	// /admin/payment_method) replaced them with a real, per-method record
	// (enabled/primary/sortOrder/image). Kept here ONLY because
	// seedPaymentMethods reads them once, on a brand-new database, to
	// carry over whatever was set here before that table existed.
	// Deliberately NOT in SettingsService.Update's whitelist anymore —
	// editing them now would silently do nothing, since nothing else
	// reads them after that one-time migration.
	CashPaymentEnabled    bool `json:"cashPaymentEnabled" gorm:"not null;default:true"`
	PPCBankPaymentEnabled bool `json:"ppcbankPaymentEnabled" gorm:"not null;default:false"`

	// Store location — set by the admin picking a point on a map (see
	// admin/settings.vue). Latitude/Longitude are the store's actual
	// coordinates; DeliveryDistanceKm is how far from that point the
	// business is willing to deliver, for the storefront to show
	// "we deliver to you" / "outside our delivery area" type messaging.
	// All default to 0, which is a real, meaningful "not set yet" state
	// (0,0 is the middle of the ocean off West Africa — never a real
	// store location), not something needing NULL/pointer handling.
	Latitude           float64 `json:"latitude" gorm:"not null;default:0"`
	Longitude          float64 `json:"longitude" gorm:"not null;default:0"`
	DeliveryDistanceKm float64 `json:"deliveryDistanceKm" gorm:"not null;default:0"`

	// ShippingFee is a flat amount added to every order's total at
	// checkout (see OrderService.buildOrderItemsFromCart). Defaults to 0
	// — a real, meaningful "no shipping charge configured" state, not
	// something needing NULL/pointer handling.
	ShippingFee float64 `json:"shippingFee" gorm:"not null;default:0"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// SettingsID is the fixed primary key of the one-and-only settings row.
const SettingsID = 1
