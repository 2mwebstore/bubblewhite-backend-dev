package models

import "time"

// AuditLog is one recorded action — either by a staff member (ActorType
// "admin") or a storefront customer (ActorType "customer"). Both actor
// types share one table rather than two, since the shape of what's
// recorded is identical either way; the admin panel's two separate log
// views (see AdminAuditLogController.List) are just this same table
// filtered by ActorType, not two different underlying systems.
//
// ActorName is a snapshot of the actor's name AT THE TIME of the action —
// deliberately denormalized (not a live join to User/Customer) so a log
// entry still reads sensibly even after the account is later renamed or
// deleted. The same reasoning applies to ResourceLabel below.
//
// ResourceID/ResourceLabel identify WHAT was acted on, when the action
// has a specific target (e.g. "updated product #42, 'Blue T-Shirt'") —
// both nullable, since some actions don't have a single target (e.g. a
// login has no resource at all).
type AuditLog struct {
	ID            uint    `json:"id" gorm:"primaryKey;autoIncrement"`
	ActorType     string  `json:"actorType" gorm:"type:varchar(20);index;not null"` // "admin" | "customer"
	ActorID       uint    `json:"actorId" gorm:"index;not null"`
	ActorName     string  `json:"actorName" gorm:"type:varchar(150)"`
	Action        string  `json:"action" gorm:"type:varchar(50);index;not null"`   // e.g. "login", "login_failed", "create", "update", "delete", "register"
	Resource      string  `json:"resource" gorm:"type:varchar(50);index;not null"` // e.g. "product", "order", "customer_profile", "auth"
	ResourceID    *string `json:"resourceId" gorm:"type:varchar(50)"`
	ResourceLabel *string `json:"resourceLabel" gorm:"type:varchar(255)"`
	Description   string  `json:"description" gorm:"type:text"`
	IPAddress     string  `json:"ipAddress" gorm:"type:varchar(64)"`
	UserAgent     string  `json:"userAgent" gorm:"type:varchar(255)"`
	// Country/IsVPN/IsProxy are filled in AFTER the row is created, by a
	// background lookup against a third-party IP intelligence API (see
	// services/ip_intelligence_service.go) — never synchronously as part
	// of Log() itself, so a slow or unreachable third-party API can
	// never add latency to the actual action being audited (a login, a
	// product update, etc.). This means these three fields may briefly
	// read as blank/false immediately after an action, before that
	// background lookup finishes — a deliberate trade-off, not a bug.
	Country   string    `json:"country" gorm:"type:varchar(100)"`
	IsVPN     bool      `json:"isVpn"`
	IsProxy   bool      `json:"isProxy"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}
