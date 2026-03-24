package features

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// CacheV2 extends SemanticCache with cross-customer shared caching and analytics.
type CacheV2 struct {
	// Per-tenant cache (uses existing SemanticCache with userID scoping)
	tenant *SemanticCache

	// Shared global cache for generic prompts (no customer-specific data)
	shared *SemanticCache

	// Analytics
	analytics *CacheAnalytics
}

// CacheAnalytics tracks cache performance metrics.
type CacheAnalytics struct {
	mu         sync.Mutex
	modelStats map[string]*modelCacheStats
	evictions  atomic.Int64
}

type modelCacheStats struct {
	Hits         int64   `json:"hits"`
	Misses       int64   `json:"misses"`
	SavedCostUSD float64 `json:"saved_cost_usd"`
}

// NewCacheV2 creates a cache with both per-tenant and shared pools.
func NewCacheV2(threshold float64, maxItems int, ttl time.Duration) *CacheV2 {
	if threshold <= 0 {
		threshold = 0.95 // Higher default for v2
	}
	return &CacheV2{
		tenant: NewSemanticCache(threshold, maxItems, ttl),
		shared: NewSemanticCache(threshold, maxItems/2, ttl*2), // Shared pool has longer TTL
		analytics: &CacheAnalytics{
			modelStats: make(map[string]*modelCacheStats),
		},
	}
}

// FindSimilar searches both tenant and shared pools.
func (c *CacheV2) FindSimilar(model string, messages []provider.Message, userID string) *CacheEntry {
	// 1. Check tenant cache first (per-user).
	entry := c.tenant.FindSimilar(model, messages, userID)
	if entry != nil {
		c.analytics.recordHit(model, entry)
		return entry
	}

	// 2. If prompt is generic, check shared cache.
	if isGenericPrompt(messages) {
		entry = c.shared.FindSimilar(model, messages) // No userID for shared
		if entry != nil {
			c.analytics.recordHit(model, entry)
			return entry
		}
	}

	c.analytics.recordMiss(model)
	return nil
}

// Store adds a response to the appropriate cache pool.
func (c *CacheV2) Store(model string, messages []provider.Message, entry *CacheEntry, userID string) {
	if isGenericPrompt(messages) {
		// Store in shared pool (available to all customers).
		c.shared.Store(model, messages, entry)
	}
	// Always store in tenant cache too.
	c.tenant.Store(model, messages, entry, userID)
}

// Stats returns combined analytics from both pools.
func (c *CacheV2) Stats() map[string]any {
	tenantStats := c.tenant.Stats()
	sharedStats := c.shared.Stats()

	c.analytics.mu.Lock()
	modelBreakdown := make(map[string]any)
	totalHits := int64(0)
	totalMisses := int64(0)
	totalSaved := 0.0
	for model, stats := range c.analytics.modelStats {
		modelBreakdown[model] = map[string]any{
			"hits":           stats.Hits,
			"misses":         stats.Misses,
			"hit_rate":       hitRate(stats.Hits, stats.Misses),
			"saved_cost_usd": stats.SavedCostUSD,
		}
		totalHits += stats.Hits
		totalMisses += stats.Misses
		totalSaved += stats.SavedCostUSD
	}
	c.analytics.mu.Unlock()

	return map[string]any{
		"tenant_cache":      tenantStats,
		"shared_cache":      sharedStats,
		"total_hits":        totalHits,
		"total_misses":      totalMisses,
		"overall_hit_rate":  hitRate(totalHits, totalMisses),
		"estimated_savings": totalSaved,
		"evictions":         c.analytics.evictions.Load(),
		"model_stats":       modelBreakdown,
	}
}

func hitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func (a *CacheAnalytics) recordHit(model string, entry *CacheEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	stats := a.getOrCreate(model)
	stats.Hits++
	// Estimate savings: rough cost per cache hit avoided.
	stats.SavedCostUSD += estimateCacheSavings(model)
}

func (a *CacheAnalytics) recordMiss(model string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	stats := a.getOrCreate(model)
	stats.Misses++
}

func (a *CacheAnalytics) getOrCreate(model string) *modelCacheStats {
	if stats, ok := a.modelStats[model]; ok {
		return stats
	}
	stats := &modelCacheStats{}
	a.modelStats[model] = stats
	return stats
}

// estimateCacheSavings returns rough per-request savings for cache hits.
func estimateCacheSavings(model string) float64 {
	// Average cost per request by model tier.
	switch {
	case strings.Contains(model, "gpt-4o-mini"), strings.Contains(model, "haiku"):
		return 0.0005
	case strings.Contains(model, "gpt-4o"), strings.Contains(model, "sonnet"):
		return 0.005
	case strings.Contains(model, "gpt-4"), strings.Contains(model, "opus"):
		return 0.02
	case strings.Contains(model, "gemini"):
		return 0.001
	default:
		return 0.002
	}
}

// --- Generic prompt classification ---

// PII and customer-specific content patterns.
var (
	piiPatternCache = []*regexp.Regexp{
		regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`),      // email
		regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),                                   // SSN
		regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),                             // credit card
		regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`), // phone
	}
	customerRefPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:my|our)\s+(?:account|order|ticket|case|subscription)\b`),
		regexp.MustCompile(`(?i)\b(?:customer|user|account)\s*(?:id|number|#)\s*[:=]?\s*\w+`),
		regexp.MustCompile(`(?i)\b(?:order|invoice|ticket)\s*#?\s*\d+\b`),
	}
	// Proper noun heuristic: capitalized words not at sentence start.
	properNounPattern = regexp.MustCompile(`\b[A-Z][a-z]{2,}\s+[A-Z][a-z]{2,}\b`)
)

// isGenericPrompt returns true if the messages contain no customer-specific data.
func isGenericPrompt(messages []provider.Message) bool {
	var allText strings.Builder
	for _, m := range messages {
		if m.Role == "user" {
			allText.WriteString(m.Content)
			allText.WriteString(" ")
		}
	}
	text := allText.String()

	// Check for PII patterns.
	for _, p := range piiPatternCache {
		if p.MatchString(text) {
			return false
		}
	}

	// Check for customer-specific references.
	for _, p := range customerRefPatterns {
		if p.MatchString(text) {
			return false
		}
	}

	// Check for proper nouns (possible person names).
	names := properNounPattern.FindAllString(text, 5)
	if len(names) > 2 {
		return false // Too many proper nouns suggests customer-specific content.
	}

	return true
}
