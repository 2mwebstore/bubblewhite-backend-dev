package config

import "time"

// JWTSecret returns the signing secret from config.
func JWTSecret() string {
	return Get().JWTSecret
}

// JWTExpiry returns the configured token lifetime.
func JWTExpiry() time.Duration {
	return time.Duration(Get().JWTExpiresHours) * time.Hour
}
