package models

import "time"

// BakongToken is a singleton row (only ever one, like Settings) caching
// the current Bakong Open API bearer token in the database — durable
// across process restarts/redeploys, unlike a purely in-memory cache, and
// correctly shared if the backend ever scales to multiple instances.
type BakongToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Token     string    `gorm:"type:text;not null"`
	FetchedAt time.Time `gorm:"not null"`
}
