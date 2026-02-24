package features

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// SemanticCache finds similar (not just identical) prompts using
// character-trigram TF-IDF vectors and cosine similarity.
// No external embedding API required — runs entirely in-process.
type SemanticCache struct {
	mu        sync.RWMutex
	entries   []semanticEntry
	threshold float64 // 0.0–1.0, default 0.85
	maxItems  int
	ttl       time.Duration
}

type semanticEntry struct {
	key       string
	vector    sparseVec
	entry     *CacheEntry
	expiresAt time.Time
}

// sparseVec is a sparse vector of trigram → TF-IDF weight.
type sparseVec map[string]float64

// NewSemanticCache creates a semantic cache.
// threshold controls how similar prompts must be to match (0.85 = 85% similar).
func NewSemanticCache(threshold float64, maxItems int, ttl time.Duration) *SemanticCache {
	if threshold <= 0 {
		threshold = 0.85
	}
	return &SemanticCache{
		threshold: threshold,
		maxItems:  maxItems,
		ttl:       ttl,
	}
}

// FindSimilar returns the best matching cache entry if similarity >= threshold.
func (sc *SemanticCache) FindSimilar(model string, messages []provider.Message) *CacheEntry {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	queryVec := vectorize(model, messages)
	now := time.Now()

	var bestEntry *CacheEntry
	var bestSim float64

	for i := range sc.entries {
		e := &sc.entries[i]
		if now.After(e.expiresAt) {
			continue
		}
		sim := cosineSimilarity(queryVec, e.vector)
		if sim >= sc.threshold && sim > bestSim {
			bestSim = sim
			bestEntry = e.entry
		}
	}

	if bestEntry != nil {
		bestEntry.Hits++
	}
	return bestEntry
}

// Store adds a response to the semantic cache.
func (sc *SemanticCache) Store(model string, messages []provider.Message, entry *CacheEntry) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Evict expired entries
	now := time.Now()
	alive := sc.entries[:0]
	for _, e := range sc.entries {
		if now.Before(e.expiresAt) {
			alive = append(alive, e)
		}
	}
	sc.entries = alive

	// Capacity check
	if len(sc.entries) >= sc.maxItems {
		return
	}

	vec := vectorize(model, messages)
	key := CacheKey(model, messages)

	sc.entries = append(sc.entries, semanticEntry{
		key:       key,
		vector:    vec,
		entry:     entry,
		expiresAt: now.Add(sc.ttl),
	})
}

// Stats returns semantic cache statistics.
func (sc *SemanticCache) Stats() map[string]any {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	totalHits := 0
	active := 0
	now := time.Now()
	for _, e := range sc.entries {
		if now.Before(e.expiresAt) {
			active++
			totalHits += e.entry.Hits
		}
	}
	return map[string]any{
		"entries":    active,
		"total_hits": totalHits,
		"threshold":  sc.threshold,
	}
}

// vectorize builds a sparse TF vector from model name + message contents.
// Uses character trigrams for language-agnostic, typo-tolerant matching.
func vectorize(model string, messages []provider.Message) sparseVec {
	// Build the text to vectorize: model + all message contents
	var parts []string
	parts = append(parts, model)
	for _, m := range messages {
		parts = append(parts, m.Role, m.Content)
	}
	text := normalize(strings.Join(parts, " "))

	// Count trigrams
	counts := make(map[string]int)
	total := 0
	runes := []rune(text)
	for i := 0; i <= len(runes)-3; i++ {
		tri := string(runes[i : i+3])
		counts[tri]++
		total++
	}

	// Also add word unigrams for better discrimination
	words := strings.Fields(text)
	for _, w := range words {
		if len(w) >= 2 {
			counts["w:"+w]++
			total++
		}
	}

	// Convert to TF (term frequency) vector
	vec := make(sparseVec, len(counts))
	ft := float64(total)
	for tri, cnt := range counts {
		vec[tri] = float64(cnt) / ft
	}
	return vec
}

// normalize lowercases and strips non-alphanumeric characters.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// cosineSimilarity computes the cosine similarity between two sparse vectors.
func cosineSimilarity(a, b sparseVec) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	var dot, normA, normB float64

	for k, va := range a {
		normA += va * va
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	for _, vb := range b {
		normB += vb * vb
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// semanticCacheKey is used for JSON marshaling in tests.
func semanticCacheKey(model string, messages []provider.Message) string {
	data, _ := json.Marshal(struct {
		Model    string             `json:"model"`
		Messages []provider.Message `json:"messages"`
	}{model, messages})
	return string(data)
}

// NewSemanticCacheFromConfig creates a SemanticCache from config.
func NewSemanticCacheFromConfig(cfg config.SemanticCacheConfig) *SemanticCache {
	threshold := cfg.Threshold
	if threshold == 0 { threshold = 0.92 }
	return NewSemanticCache(threshold, 1000, 5*time.Minute)
}

// SemanticCacheMiddleware wraps SemanticCache as proxy middleware.
func SemanticCacheMiddleware(sc *SemanticCache) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Check cache
			if cached := sc.FindSimilar(req.Model, req.Messages); cached != nil {
				return cached.Response, nil
			}
			resp, err := next(ctx, req)
			if err == nil && resp != nil {
				sc.Store(req.Model, req.Messages, &CacheEntry{Response: resp})
			}
			return resp, err
		}
	}
}
