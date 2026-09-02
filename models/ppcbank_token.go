package models

import "time"

// PPCBankToken is a singleton row (only ever one, like Settings and
// BakongToken) caching the current PPCBank Payment Gateway bearer token in
// the database — durable across process restarts/redeploys.
type PPCBankToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Token     string    `gorm:"type:text;not null"`
	FetchedAt time.Time `gorm:"not null"`
}
