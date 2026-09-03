package models

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	PaymentMethodCash    = "cash"
	PaymentMethodBakong  = "bakong" // historical only — Bakong integration removed, this remains solely so existing orders still display correctly
	PaymentMethodPPCBank = "ppcbank"
)

const (
	OrderStatusPending   = "pending"
	OrderStatusConfirmed = "confirmed"
	OrderStatusShipped   = "shipped"
	OrderStatusCompleted = "completed"
	OrderStatusCancelled = "cancelled"
)

const (
	PaymentStatusUnpaid = "unpaid"
	PaymentStatusPaid   = "paid"
	PaymentStatusFailed = "failed"
)

// Order is a checked-out cart. Items is a GORM association (not just a
// foreign key) so callers can Preload("Items") to get the full order in
// one query for detail views.
//
// Address and Phone are snapshotted at checkout time — the customer picks/
// confirms these per order (defaulting to their account phone, but
// editable), so a later profile change must never alter what a past order
// actually shipped to.
//
// PaymentStatus is deliberately separate from Status (order fulfillment
// lifecycle) — a cash order can be "confirmed" while still "unpaid" (pay
// on delivery), and a PPCBank order's payment can be verified independent
// of where the order is in fulfillment. PaymentReference holds whatever
// identifier the active payment method needs for later verification (for
// PPCBank, this is the order's own billNumber — see
// OrderService.VerifyPPCBankPayment), not just trust what the customer's
// browser reported during checkout.
type Order struct {
	ID         uint    `json:"id" gorm:"primaryKey;autoIncrement"`
	CustomerID uint    `json:"customerId" gorm:"index;not null"`
	Total      float64 `json:"total" gorm:"not null"`
	// ShippingFee snapshots Settings.ShippingFee AT THE TIME this order
	// was placed — not recomputed from current settings later, same
	// reasoning as OrderItem snapshotting product price/name: if the
	// admin changes the shipping fee tomorrow, an order placed today
	// must still show what was actually charged, not today's new rate.
	// Total already includes this amount; it's stored separately purely
	// for transparency on receipts/admin view.
	ShippingFee      float64 `json:"shippingFee" gorm:"not null;default:0"`
	PaymentMethod    string  `json:"paymentMethod" gorm:"type:varchar(20);not null"` // "cash" | "bakong"
	PaymentStatus    string  `json:"paymentStatus" gorm:"type:varchar(20);not null;default:unpaid"`
	PaymentReference string  `json:"paymentReference" gorm:"type:varchar(50)"` // KHQR MD5 hash, empty for cash orders
	// TransactionHash is Bakong's own full transaction hash — captured
	// from the successful MD5 check's response at checkout time (the MD5
	// check itself returns this, we just weren't storing it before). Used
	// for the admin's richer check_transaction_by_hash lookup, which
	// returns tracking status and receiver bank detail that the MD5 check
	// doesn't. Empty until a Bakong payment is actually verified.
	TransactionHash string `json:"transactionHash" gorm:"type:varchar(100)"`
	// Invoice is the bank's own reference number for this payment —
	// PPCBank's Check KHQR Payment Status (PMS1024) returns this as
	// referenceNo once a payment is confirmed paid (e.g. "20671600").
	// Distinct from TransactionHash: that's a cross-bank Bakong routing
	// hash, this is PPCBank's own internal transaction ID — the number
	// that would actually show up if reconciling against a real PPCBank
	// statement. Empty until a PPCBank payment is verified paid.
	Invoice   string      `json:"invoice" gorm:"type:varchar(100)"`
	Status    string      `json:"status" gorm:"type:varchar(20);not null;default:pending"`
	Address   string      `json:"address" gorm:"type:text;not null"`
	Phone     string      `json:"phone" gorm:"type:varchar(50);not null"`
	Items     []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// Reference is the order's human-friendly reference number — e.g.
// "BW-000042" — for customer support, printed/shared confirmations, and
// generally anywhere a raw database ID would be awkward to quote out loud
// or over chat. Deliberately a METHOD, not a stored column: it's fully
// derivable from ID, so computing it on demand means it can never drift
// out of sync with the actual order (no migration, no backfill, no risk
// of the stored value and the real ID ever disagreeing).
func (o Order) Reference() string {
	return fmt.Sprintf("BW-%06d", o.ID)
}

// ParseOrderReference is the inverse of Reference() — parses "BW-000042"
// back into its numeric order ID. Used by the PPCBank webhook handler,
// which only receives the billNumber (= our Reference()) in its payload,
// never our internal order ID directly.
func ParseOrderReference(ref string) (uint, error) {
	var id uint
	n, err := fmt.Sscanf(ref, "BW-%d", &id)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("invalid order reference: %q", ref)
	}
	return id, nil
}

// MarshalJSON adds the computed Reference field to every JSON
// representation of an Order — single or within a slice (json.Marshal
// calls this per-element automatically) — without requiring every
// controller/service call site that returns an order to remember to
// attach it separately.
func (o Order) MarshalJSON() ([]byte, error) {
	type alias Order // avoids infinite recursion into this same MarshalJSON
	return json.Marshal(struct {
		alias
		Reference string `json:"reference"`
	}{
		alias:     alias(o),
		Reference: o.Reference(),
	})
}

// OrderItem snapshots the product's name/price/image/size at the moment of
// checkout — deliberately NOT a live reference to Product, since a later
// price change or product deletion must never alter what a past order
// actually showed/charged.
type OrderItem struct {
	ID        uint    `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID   uint    `json:"orderId" gorm:"index;not null"`
	ProductID string  `json:"productId" gorm:"type:varchar(50);not null"`
	Name      string  `json:"name" gorm:"type:varchar(255);not null"`
	Price     float64 `json:"price" gorm:"not null"`
	Image     string  `json:"image" gorm:"type:varchar(500)"`
	Size      string  `json:"size" gorm:"type:varchar(20)"`
	Quantity  int     `json:"quantity" gorm:"not null"`
}
