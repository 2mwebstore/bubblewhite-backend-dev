package utils

import (
	"errors"
	"time"

	"bubblewhite-backend/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the payload embedded in every access token this app issues.
type Claims struct {
	UserID uint   `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"` // role slug, e.g. "admin"
	jwt.RegisteredClaims
}

// SignToken creates a signed JWT for the given user/role.
func SignToken(userID uint, email, roleSlug string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   roleSlug,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.JWTExpiry())),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSecret()))
}

// ParseToken validates a token string and returns its claims.
func ParseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.JWTSecret()), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// CustomerClaims is the payload for storefront customer tokens — kept as a
// separate type from admin's Claims (rather than reusing it with a
// sentinel Role value) so a customer token can never accidentally satisfy
// an admin RequirePermission check, and vice versa.
type CustomerClaims struct {
	CustomerID uint   `json:"customerId"`
	Identifier string `json:"identifier"` // whatever they logged in/registered with — phone or email, informational only
	jwt.RegisteredClaims
}

// SignCustomerToken creates a signed JWT for a logged-in storefront customer.
func SignCustomerToken(customerID uint, identifier string) (string, error) {
	claims := CustomerClaims{
		CustomerID: customerID,
		Identifier: identifier,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.JWTExpiry())),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSecret()))
}

// ParseCustomerToken validates a customer token string and returns its claims.
func ParseCustomerToken(tokenString string) (*CustomerClaims, error) {
	claims := &CustomerClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.JWTSecret()), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
