package features

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// CacheConfig defines caching settings.
type CacheConfig struct {
	Enabled    bool
	Strategy   string        // "exact" or "semantic"
	TTL        time.Duration
	MaxEntries int
}

// CacheEntry holds a cached response.
type CacheEntry struct {
	Response   *provider.Response
	Model      string
	TokensSaved int
	CostSaved  float64
	Hits       int
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// Cache is an in-memory exact-match cache backed by SQLite.
type Cache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	config   CacheConfig
	semantic *SemanticCache // nil when strategy != "semantic"
}

// NewCache creates a new cache instance.
func NewCache(cfg CacheConfig) *Cache {
	c := &Cache{
		entries: make(map[string]*CacheEntry),
		config:  cfg,
	}
	if cfg.Strategy == "semantic" {
		c.semantic = NewSemanticCache(0.85, cfg.MaxEntries, cfg.TTL)
	}
	return c
}

// CacheKey generates a deterministic key from model + messages.
func CacheKey(model string, messages []provider.Message) string {
	data, _ := json.Marshal(struct {
		Model    string             `json:"model"`
		Messages []provider.Message `json:"messages"`
	}{model, messages})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response, or nil if not found/expired.
func (c *Cache) Get(key string) *CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}
	entry.Hits++
	return entry
}

// Set stores a response in the cache.
func (c *Cache) Set(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity (simple: just skip — proper LRU in v1.1)
	if len(c.entries) >= c.config.MaxEntries {
		return
	}
	c.entries[key] = entry
}

// Stats returns cache statistics.
func (c *Cache) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalHits := 0
	totalSavings := 0.0
	for _, e := range c.entries {
		totalHits += e.Hits
		totalSavings += e.CostSaved * float64(e.Hits)
	}
	result := map[string]any{
		"entries":     len(c.entries),
		"total_hits":  totalHits,
		"savings_usd": totalSavings,
		"strategy":    c.config.Strategy,
	}
	if c.semantic != nil {
		result["semantic"] = c.semantic.Stats()
	}
	return result
}

// Clear removes all cache entries.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
	if c.semantic != nil {
		c.semantic.mu.Lock()
		c.semantic.entries = nil
		c.semantic.mu.Unlock()
	}
}

// CacheMiddleware returns middleware that checks/populates the cache.
// Supports two strategies:
//   - "exact": SHA-256 hash of model+messages (fast, deterministic)
//   - "semantic": trigram cosine similarity (catches paraphrases)
func CacheMiddleware(cache *Cache) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if req.Stream {
				return next(ctx, req)
			}

			// Step 1: Always try exact match first (fast path)
			key := CacheKey(req.Model, req.Messages)
			if entry := cache.Get(key); entry != nil {
				resp := *entry.Response
				resp.CacheHit = true
				return &resp, nil
			}

			// Step 2: Try semantic match if enabled
			if cache.semantic != nil {
				if entry := cache.semantic.FindSimilar(req.Model, req.Messages); entry != nil {
					resp := *entry.Response
					resp.CacheHit = true
					return &resp, nil
				}
			}

			// Step 3: Cache miss — call provider
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Step 4: Store in both caches
			entry := &CacheEntry{
				Response:  resp,
				Model:     req.Model,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(cache.config.TTL),
			}
			cache.Set(key, entry)
			if cache.semantic != nil {
				cache.semantic.Store(req.Model, req.Messages, entry)
			}

			return resp, nil
		}
	}
}
