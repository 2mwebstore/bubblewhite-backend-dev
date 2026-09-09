package services

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"bubblewhite-backend/repositories"
)

// GeoInfo is the result of one IP lookup — the three fields worth
// storing on an audit log entry. Zero values (empty Country, both flags
// false) mean "unknown/not looked up", not "confirmed not a VPN" — see
// IPIntelligenceService.Lookup's own doc comment for the various cases
// that produce this.
type GeoInfo struct {
	Country string
	IsVPN   bool
	IsProxy bool
}

// IPIntelligenceService wraps IPLocate.io (iplocate.io) — chosen
// specifically because its free tier (1,000 lookups/day, no card
// required) returns is_vpn and is_proxy as two SEPARATE fields, rather
// than one conflated "anonymizer" flag the way some other free
// geolocation APIs do; that separation is what was actually asked for.
type IPIntelligenceService struct {
	Settings *repositories.SettingsRepository
	client   *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	info      GeoInfo
	expiresAt time.Time
}

// cacheTTL: audit-worthy admin/customer sessions are often several
// actions in a row from the same IP within a short window — caching
// avoids burning through the free tier's daily quota re-looking-up the
// same IP for every single one of those actions.
const geoCacheTTL = 1 * time.Hour

func NewIPIntelligenceService(settings *repositories.SettingsRepository) *IPIntelligenceService {
	return &IPIntelligenceService{
		Settings: settings,
		client:   &http.Client{Timeout: 5 * time.Second},
		cache:    make(map[string]cacheEntry),
	}
}

type iplocateResponse struct {
	Country *string `json:"country"`
	Privacy struct {
		IsVPN   bool `json:"is_vpn"`
		IsProxy bool `json:"is_proxy"`
	} `json:"privacy"`
}

// Lookup returns country/VPN/proxy info for ip, or a zero-value GeoInfo
// (never an error) in any of these cases — all treated the same way,
// since this is best-effort enrichment and no caller should ever be
// blocked or fail because of it:
//   - ip is empty, unparseable, private, or loopback (nothing meaningful
//     an external API could ever say about a LAN or local address —
//     this also keeps local development from wasting API quota on every
//     single logged action)
//   - no IPIntelligenceAPIKey is configured in Settings
//   - the third-party API errors, times out, or returns something
//     unexpected
func (s *IPIntelligenceService) Lookup(ip string) GeoInfo {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsLoopback() || parsed.IsPrivate() {
		return GeoInfo{}
	}

	s.mu.Lock()
	if entry, ok := s.cache[ip]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.Unlock()
		return entry.info
	}
	s.mu.Unlock()

	settings, err := s.Settings.Get()
	if err != nil || settings.IPIntelligenceAPIKey == "" {
		return GeoInfo{}
	}

	info := s.fetch(ip, settings.IPIntelligenceAPIKey)

	s.mu.Lock()
	s.cache[ip] = cacheEntry{info: info, expiresAt: time.Now().Add(geoCacheTTL)}
	s.mu.Unlock()

	return info
}

func (s *IPIntelligenceService) fetch(ip, apiKey string) GeoInfo {
	reqURL := "https://iplocate.io/api/lookup/" + url.PathEscape(ip) + "?apikey=" + url.QueryEscape(apiKey)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return GeoInfo{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GeoInfo{}
	}

	var parsed iplocateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return GeoInfo{}
	}

	country := ""
	if parsed.Country != nil {
		country = *parsed.Country
	}
	return GeoInfo{Country: country, IsVPN: parsed.Privacy.IsVPN, IsProxy: parsed.Privacy.IsProxy}
}
