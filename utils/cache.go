package utils

import (
	"sync"
	"time"
)

// Cache is a simple, thread-safe, in-memory TTL cache — deliberately
// generic-value (interface{}) rather than typed per use, since the
// handful of things worth caching in this app (settings, category
// lists, product lists/details) are different shapes, and each caller
// already knows what type it stored so a type assertion at the call
// site is no real burden.
//
// In-memory, not Redis: this backend runs as a single instance on
// Railway (see ARCHITECTURE.md §1) — there's no second process that
// would need a shared cache, and introducing Redis would add a new
// service, a new cost, and a new failure mode to solve a scaling
// problem that doesn't exist yet. If the backend is ever horizontally
// scaled, this is the piece that would need to move to Redis at that
// point — not before.
//
// Each service that uses this should hold its OWN *Cache instance
// (constructed once via NewCache and stored as a field), not share one
// global instance across unrelated services — that way a product write
// clearing its cache can never accidentally wipe cached settings or
// categories too.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func NewCache() *Cache {
	return &Cache{items: make(map[string]cacheItem)}
}

// Get returns the cached value and true if present and not expired. An
// expired-but-still-present entry is treated as a miss — it isn't
// eagerly deleted here, since Set overwrites it on the next successful
// fetch anyway and a background sweep isn't worth the added complexity
// for the small number of keys this cache actually holds in this app.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheItem{value: value, expiresAt: time.Now().Add(ttl)}
}

// Delete removes one specific key — used for single-record invalidation
// (e.g. the one settings row, one product by ID).
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes every entry in this cache instance — used when a write
// could plausibly affect many cached keys at once (e.g. a product list
// cached under many different filter/pagination combinations), where
// tracking exactly which keys are affected isn't worth the bookkeeping
// compared to clearing everything and letting the next read repopulate
// it. Safe to call broadly BECAUSE each service holds its own Cache
// instance — this never touches another service's cached data.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}
