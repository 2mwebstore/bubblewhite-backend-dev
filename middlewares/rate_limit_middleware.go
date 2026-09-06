package middlewares

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// visitor tracks one client's recent request activity via a token bucket:
// tokens refill continuously at `rate` per second, each request consumes
// one token, and requests are rejected once the bucket is empty. This
// allows normal bursty browsing (loading a page pulls in several API
// calls at once) while still capping sustained abuse.
type visitor struct {
	tokens   float64
	lastSeen time.Time
}

type RateLimiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	rate      float64 // tokens added per second
	burst     float64 // max tokens a bucket can ever hold
	message   string
	skipPaths []string
	allowlist map[string]bool
	// name identifies this limiter in logs (e.g. "login", "contact",
	// "global") — otherwise a blocked-request log line can't tell you
	// WHICH limit was hit, just that something was.
	name string
}

// NewRateLimiter creates a limiter allowing `requestsPerMinute` sustained,
// with short bursts up to `burst` tokens. No external dependency — this
// project can't reliably fetch new Go modules in every environment it's
// built in, and a limiter this size doesn't need one.
func NewRateLimiter(requestsPerMinute int, burst int, message string) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     float64(requestsPerMinute) / 60.0,
		burst:    float64(burst),
		message:  message,
		name:     "unnamed",
	}
	go rl.cleanupLoop()
	return rl
}

// Name sets the identifier used in this limiter's own log lines — always
// call this right after NewRateLimiter for every limiter you create, so
// "rate limit: blocked" log entries are actually traceable back to which
// limit (login, contact, global, ...) was hit.
func (rl *RateLimiter) Name(name string) *RateLimiter {
	rl.name = name
	return rl
}

// SkipPaths exempts the given path PREFIXES from this limiter entirely.
// Used for the PPCBank webhook: every call comes from PPCBank's own
// servers, not arbitrary users, and a busy sales period could otherwise
// mean many real payment confirmations arriving close together from that
// single source IP — tripping a general-purpose limiter and causing
// legitimate payments to fail verification, not blocking an attacker.
// That endpoint already has its own trust model anyway (see
// PPCBankWebhookController.Notify, which never trusts the payload alone
// and independently re-verifies against PPCBank's real API) — it was
// never relying on rate limiting for security in the first place.
func (rl *RateLimiter) SkipPaths(paths ...string) *RateLimiter {
	rl.skipPaths = paths
	return rl
}

func (rl *RateLimiter) shouldSkip(path string) bool {
	for _, p := range rl.skipPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Allowlist exempts the given IPs from this limiter entirely — e.g. your
// own office IP, or a monitoring/health-check service that legitimately
// calls the API more often than any real user would. Checked before the
// token bucket, so an allowlisted IP never consumes tokens or shows up in
// the visitors map at all.
func (rl *RateLimiter) Allowlist(ips ...string) *RateLimiter {
	rl.allowlist = make(map[string]bool, len(ips))
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			rl.allowlist[ip] = true
		}
	}
	return rl
}

func (rl *RateLimiter) isAllowlisted(ip string) bool {
	return rl.allowlist[ip]
}

// cleanupLoop periodically forgets visitors that haven't been seen in a
// while — without this, the visitors map would grow forever as new IPs
// show up, an unbounded-memory-growth bug that's its own kind of DoS
// vulnerability if left unchecked.
func (rl *RateLimiter) cleanupLoop() {
	for {
		time.Sleep(5 * time.Minute)
		cutoff := time.Now().Add(-15 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// checkResult carries enough detail to set accurate rate-limit response
// headers, not just a plain allow/deny bool.
type checkResult struct {
	allowed        bool
	remaining      int
	retryAfterSecs int // only meaningful when allowed is false
}

func (rl *RateLimiter) check(ip string) checkResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{tokens: rl.burst - 1, lastSeen: now}
		return checkResult{allowed: true, remaining: int(rl.burst - 1)}
	}

	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}
	v.lastSeen = now

	if v.tokens < 1 {
		// Exact time until 1 token becomes available, given the current
		// deficit and refill rate — not a flat guess, so a client that
		// actually respects Retry-After waits the right amount, no more
		// and no less.
		deficit := 1 - v.tokens
		retryAfter := int(deficit/rl.rate) + 1
		return checkResult{allowed: false, remaining: 0, retryAfterSecs: retryAfter}
	}

	v.tokens -= 1
	return checkResult{allowed: true, remaining: int(v.tokens)}
}

// Middleware returns a gin.HandlerFunc enforcing this limiter's rate
// against realClientIP(c) — never c.ClientIP() directly, see that
// function's own comment for why.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.shouldSkip(c.Request.URL.Path) {
			c.Next()
			return
		}

		ip := realClientIP(c)
		if rl.isAllowlisted(ip) {
			c.Next()
			return
		}

		result := rl.check(ip)

		// Set on every response, not just blocked ones — lets a
		// well-behaved client (including your own frontend) see how
		// close it is to the limit and back off before actually hitting
		// it, and makes this visible in the browser network tab or curl
		// while debugging a threshold that might be tuned too tight.
		c.Header("X-RateLimit-Limit", strconv.Itoa(int(rl.burst)))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.remaining))

		if !result.allowed {
			c.Header("Retry-After", strconv.Itoa(result.retryAfterSecs))
			log.Printf("rate limit [%s]: blocked ip=%s path=%s", rl.name, ip, c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   rl.message,
			})
			return
		}
		c.Next()
	}
}

// realClientIP extracts the actual client IP behind Railway's edge proxy.
//
// This deliberately does NOT use Gin's built-in c.ClientIP(), which
// requires correctly configuring SetTrustedProxies with Railway's proxy
// IP ranges — ranges Railway's own support forum describes as shifting
// as they roll out new CDN infrastructure, making a hardcoded CIDR list
// a real maintenance risk (see Railway Central Station, "Which header
// should I rely on for real client IP?", March 2026).
//
// Instead, this follows Railway's own current, explicit guidance from
// that same thread: their edge proxy controls X-Forwarded-For and always
// prepends the real connecting IP as the first entry — a client sending
// a forged X-Forwarded-For cannot make their own request appear to come
// from a different IP, since Railway's edge adds the true IP in front of
// whatever the client sent, not after. So the first/leftmost entry is
// safe to trust here specifically because it comes from Railway's proxy,
// not the client. X-Real-Ip is deliberately NOT used: the same Railway
// thread describes an open, acknowledged bug where it gets overwritten
// with their CDN's edge IP instead of the true client IP once their CDN
// layer is in the request path.
//
// Falls back to Gin's own ClientIP() (the raw TCP remote address) when
// there's no proxy in front at all — e.g. local development.
func realClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" {
			return first
		}
	}
	return c.ClientIP()
}
