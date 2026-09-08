package services

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GoogleUser is the verified identity extracted from a Google ID token —
// only what CustomerService actually needs, not the full claim set.
type GoogleUser struct {
	Sub   string // Google's stable, unique user ID — what GoogleID links against, never the email (a Google account's email can change)
	Email string
	Name  string
}

type googleClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	jwt.RegisteredClaims
}

// googleJWK mirrors one entry of Google's certs endpoint response —
// https://www.googleapis.com/oauth2/v3/certs. Only the fields needed to
// reconstruct an rsa.PublicKey are kept.
type googleJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"` // RSA modulus, base64url, unpadded
	E   string `json:"e"` // RSA exponent, base64url, unpadded
}

type GoogleOAuthService struct {
	clientID string

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewGoogleOAuthService(clientID string) *GoogleOAuthService {
	return &GoogleOAuthService{clientID: clientID}
}

var ErrGoogleNotConfigured = errors.New("google sign-in is not configured")
var ErrInvalidGoogleToken = errors.New("invalid google token")

// keysMaxAge governs how long a fetched set of Google's public keys is
// trusted before being re-fetched. Google itself documents that these
// keys are "regularly rotated" via the response's own Cache-Control
// header, but a fixed, conservative refresh window is far simpler and
// safer than parsing and honoring that header ourselves, at the cost of
// (at most) this much delay before a freshly-rotated key is recognized.
const keysMaxAge = 1 * time.Hour

// getKey returns the RSA public key for the given key ID, fetching and
// caching Google's full key set if it's missing or stale. Locked for the
// whole operation (not just the cache read) — a thundering herd of
// concurrent logins during a cache miss should refetch once, not once per
// request.
func (g *GoogleOAuthService) getKey(kid string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if key, ok := g.keys[kid]; ok && time.Since(g.fetchedAt) < keysMaxAge {
		return key, nil
	}

	if err := g.refreshKeysLocked(); err != nil {
		// A transient network issue fetching Google's certs shouldn't
		// fail every login outright if we already have a (now
		// past-TTL, but not necessarily wrong) key for this exact kid
		// from a previous successful fetch — Google's signing keys
		// aren't revoked the instant they're rotated out, they stay
		// valid for a grace period specifically so in-flight tokens
		// keep verifying. Falling back here trades a small amount of
		// key-rotation staleness for not taking login down every time
		// this one outbound call has a bad moment.
		if key, ok := g.keys[kid]; ok {
			return key, nil
		}
		return nil, err
	}

	key, ok := g.keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: no matching key for kid %q", ErrInvalidGoogleToken, kid)
	}
	return key, nil
}

func (g *GoogleOAuthService) refreshKeysLocked() error {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v3/certs")
	if err != nil {
		return fmt.Errorf("fetching google public keys: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading google public keys response: %w", err)
	}

	var parsed struct {
		Keys []googleJWK `json:"keys"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parsing google public keys: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(parsed.Keys))
	for _, jwk := range parsed.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSAPublicKey(jwk)
		if err != nil {
			continue // skip a single malformed key rather than fail the whole refresh
		}
		keys[jwk.Kid] = pub
	}

	g.keys = keys
	g.fetchedAt = time.Now()
	return nil
}

// jwkToRSAPublicKey reconstructs an rsa.PublicKey from a JWK's base64url
// modulus (n) and exponent (e) — the same encoding jwt.io and every other
// JWKS consumer expects; RawURLEncoding specifically because JWK values
// are unpadded base64url, not standard base64.
func jwkToRSAPublicKey(jwk googleJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

// VerifyIDToken verifies a Google ID token's signature and claims, and
// returns the identity it asserts. Never trust an ID token's claims
// without this — anyone can construct a JWT-shaped string claiming to be
// any email; only a valid signature from Google's own keys, over the
// exact audience this app expects, makes that claim trustworthy.
func (g *GoogleOAuthService) VerifyIDToken(idToken string) (*GoogleUser, error) {
	if g.clientID == "" {
		return nil, ErrGoogleNotConfigured
	}

	claims := &googleClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method", ErrInvalidGoogleToken)
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("%w: token has no kid header", ErrInvalidGoogleToken)
		}
		return g.getKey(kid)
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidGoogleToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidGoogleToken
	}

	// jwt.ParseWithClaims already rejects an expired exp — these are the
	// checks it does NOT do on its own, and skipping any one of them
	// would mean accepting a token this app was never the intended
	// audience for, or one issued by something other than Google itself.
	if claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com" {
		return nil, fmt.Errorf("%w: unexpected issuer %q", ErrInvalidGoogleToken, claims.Issuer)
	}
	validAudience := false
	for _, aud := range claims.Audience {
		if aud == g.clientID {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return nil, fmt.Errorf("%w: token was not issued for this app", ErrInvalidGoogleToken)
	}
	if !claims.EmailVerified {
		return nil, fmt.Errorf("%w: email not verified by google", ErrInvalidGoogleToken)
	}
	if claims.Sub == "" || claims.Email == "" {
		return nil, fmt.Errorf("%w: missing sub or email claim", ErrInvalidGoogleToken)
	}

	return &GoogleUser{Sub: claims.Sub, Email: claims.Email, Name: claims.Name}, nil
}
