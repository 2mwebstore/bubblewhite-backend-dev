package services

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// GeoInfo is the result of one IP lookup. An empty Country means
// "unknown/not looked up" — see Lookup's own doc comment for the
// various cases that produce this.
type GeoInfo struct {
	Country string
}

// IPIntelligenceService wraps ip-api.com's free geolocation endpoint —
// country only (VPN/proxy detection was dropped from this feature by
// request). Deliberately chosen over a paid/keyed service now that only
// country is needed: ip-api.com's free tier requires no signup, no API
// key, and no Settings configuration at all — one less setup step for
// something this simple.
type IPIntelligenceService struct {
	client *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	info      GeoInfo
	expiresAt time.Time
}

// cacheTTL: audit-worthy admin/customer sessions are often several
// actions in a row from the same IP within a short window — caching
// avoids a repeat lookup for every single one of those actions.
const geoCacheTTL = 1 * time.Hour

func NewIPIntelligenceService() *IPIntelligenceService {
	return &IPIntelligenceService{
		client: &http.Client{Timeout: 5 * time.Second},
		cache:  make(map[string]cacheEntry),
	}
}

type ipAPIResponse struct {
	Status  string `json:"status"`
	Country string `json:"country"`
}

// Lookup returns country info for ip, or a zero-value GeoInfo (never an
// error) in any of these cases — all treated the same way, since this is
// best-effort enrichment and no caller should ever be blocked or fail
// because of it:
//   - ip is empty, unparseable, private, or loopback (nothing meaningful
//     an external API could ever say about a LAN or local address —
//     this also keeps local development from wasting lookups on every
//     single logged action)
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

	info := s.fetch(ip)

	s.mu.Lock()
	s.cache[ip] = cacheEntry{info: info, expiresAt: time.Now().Add(geoCacheTTL)}
	s.mu.Unlock()

	return info
}

func (s *IPIntelligenceService) fetch(ip string) GeoInfo {
	// ip-api.com's free tier is HTTP-only (not HTTPS) — this is an
	// outbound server-to-server call this backend makes to a third
	// party, not something exposed to any end user's own browser
	// connection, so it carries none of the risk plain HTTP would in a
	// user-facing context.
	reqURL := "http://ip-api.com/json/" + ip + "?fields=status,country"
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return GeoInfo{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GeoInfo{}
	}

	var parsed ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || parsed.Status != "success" {
		return GeoInfo{}
	}
	return GeoInfo{Country: parsed.Country}
}
