package utils

import (
	"math"
	"strings"

	"github.com/google/uuid"
)

// RoundMoney rounds a float64 to exactly 2 decimal places — standard
// currency precision. Ordinary float64 arithmetic (summing several cart
// item prices, adding a shipping fee, etc.) routinely produces values
// like 30.490000000000002 due to how binary floating point represents
// decimal fractions — this is not a bug in that arithmetic, it's how
// IEEE 754 floats fundamentally work, but external payment APIs (PPCBank
// confirmed to reject amounts like that outright: "Amount is invalid",
// resultCode 000001) and a database column both expect a clean 2-decimal
// value. Call this anywhere a computed money total crosses a boundary —
// stored in the database, sent to an external API, or displayed.
func RoundMoney(amount float64) float64 {
	return math.Round(amount*100) / 100
}

// Ptr returns a pointer to v — handy for optional struct fields like
// Product.CompareAt (*float64) or Product.Badge (*string) in literals.
func Ptr[T any](v T) *T {
	return &v
}

// NewID generates a short, URL-safe unique ID (first 8 hex chars of a UUIDv4)
// for resources that use human-scannable string IDs, e.g. products.
// Prefix with a resource tag for readability: NewID("bw") -> "bw-a1b2c3d4".
func NewID(prefix string) string {
	raw := strings.ReplaceAll(uuid.NewString(), "-", "")
	if prefix == "" {
		return raw[:12]
	}
	return prefix + "-" + raw[:8]
}

// SortDirection normalizes a client-supplied sort direction to "ASC"/"DESC",
// defaulting to ASC for anything unrecognized.
func SortDirection(raw string) string {
	if strings.EqualFold(raw, "desc") {
		return "DESC"
	}
	return "ASC"
}

// AllowedSortColumn only lets through column names present in `allowed`,
// preventing SQL injection via a client-controlled ?sortBy= value.
func AllowedSortColumn(requested string, allowed []string, fallback string) string {
	for _, a := range allowed {
		if a == requested {
			return requested
		}
	}
	return fallback
}
