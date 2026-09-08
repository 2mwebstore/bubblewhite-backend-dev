package utils

import (
	"fmt"
	"regexp"
)

var nonDigits = regexp.MustCompile(`\D`)

// LocalVariant converts a normalized phone (e.g. "85512345678") back to
// the local, 0-prefixed form ("012345678") a customer may have originally
// registered with, before this project normalized phone numbers
// consistently. Used only as a fallback lookup in
// CustomerService.LoginWithVerifiedPhone: an existing account whose Phone
// was stored in this older, unnormalized shape would otherwise
// incorrectly look like "no account" the first time its owner tries OTP
// login, purely because of a storage-format mismatch that predates this
// feature — not because they don't actually have an account.
func LocalVariant(normalized string) string {
	if len(normalized) > 3 && normalized[:3] == "855" {
		return "0" + normalized[3:]
	}
	return normalized
}

// NormalizeCambodianPhone converts any of the common ways a Cambodian
// phone number gets typed into one consistent form: 855 followed by the
// subscriber number, digits only, no +, no leading 0.
//
//	"012345678"       -> "85512345678"  (local format, leading 0 dropped)
//	"+855 12 345 678" -> "85512345678"  (spaces/dashes stripped, + dropped)
//	"85512345678"     -> "85512345678"  (already normalized, unchanged)
//
// This exact format is what Plasgate's SMS API requires for the `to`
// field (see services/sms_plasgate.go) — but the reason this exists as a
// shared utility rather than being inlined there is that it ALSO governs
// how phone numbers are stored/compared everywhere in the OTP flow
// (OtpRequest.Phone, TelegramPhoneLink.Phone) and now in
// CustomerService's own Register/UpdateProfile too, applied consistently.
// A customer who types "012345678" when registering and "85512345678"
// when logging in via OTP six months later needs both to resolve to the
// exact same stored value, or the two "found no matching account" checks
// this project relies on (see CustomerService.LoginWithVerifiedPhone,
// ErrNoAccountForPhone) would incorrectly treat them as different people.
//
// Returns an error for anything that doesn't look like a plausible
// Cambodian mobile number (8 or 9 digits after the country code) —
// deliberately loose on exact length, since Cambodia has used both 8- and
// 9-digit subscriber numbers across different carriers/eras, but tight
// enough to catch an obviously wrong input (too short, non-numeric)
// before it reaches Plasgate or gets silently stored as garbage.
func NormalizeCambodianPhone(input string) (string, error) {
	digits := nonDigits.ReplaceAllString(input, "")

	if len(digits) > 0 && digits[0] == '0' {
		digits = "855" + digits[1:]
	}

	if len(digits) < 3 || digits[:3] != "855" {
		return "", fmt.Errorf("phone number must be a valid Cambodian number")
	}

	subscriberLen := len(digits) - 3
	if subscriberLen < 8 || subscriberLen > 9 {
		return "", fmt.Errorf("phone number must be a valid Cambodian number")
	}

	return digits, nil
}
