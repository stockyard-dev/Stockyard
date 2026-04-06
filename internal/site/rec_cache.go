package site

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

const recCacheSchema = `
CREATE TABLE IF NOT EXISTS recommendation_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    normalized_input TEXT UNIQUE NOT NULL,
    bundle_slug TEXT NOT NULL DEFAULT '',
    result_json TEXT NOT NULL,
    hit_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_hit TEXT
);
CREATE INDEX IF NOT EXISTS idx_rc_normalized ON recommendation_cache(normalized_input);
CREATE INDEX IF NOT EXISTS idx_rc_hits ON recommendation_cache(hit_count DESC);
`

// RecCache is the Layer 2 normalized-input cache backed by SQLite.
type RecCache struct {
	db *sql.DB
}

// NewRecCache creates the cache table and returns a cache handle.
func NewRecCache(db *sql.DB) *RecCache {
	for _, stmt := range strings.Split(recCacheSchema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			db.Exec(stmt)
		}
	}
	return &RecCache{db: db}
}

// Get looks up a cached recommendation by normalized input or bundle slug.
func (c *RecCache) Get(key string) (*RecommendResult, bool) {
	var resultJSON string
	// Try normalized_input first, then bundle_slug
	err := c.db.QueryRow(
		"SELECT result_json FROM recommendation_cache WHERE normalized_input = ? OR bundle_slug = ? LIMIT 1",
		key, key,
	).Scan(&resultJSON)

	if err != nil {
		return nil, false
	}

	// Update hit stats
	c.db.Exec(
		"UPDATE recommendation_cache SET hit_count = hit_count + 1, last_hit = datetime('now') WHERE normalized_input = ? OR bundle_slug = ?",
		key, key,
	)

	var result RecommendResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, false
	}

	return &result, true
}

// Set stores a recommendation result in the cache.
func (c *RecCache) Set(normalized string, slug string, result *RecommendResult) {
	data, _ := json.Marshal(result)
	c.db.Exec(
		`INSERT INTO recommendation_cache (normalized_input, bundle_slug, result_json)
         VALUES (?, ?, ?)
         ON CONFLICT(normalized_input) DO UPDATE SET
            result_json = excluded.result_json,
            hit_count = hit_count + 1,
            last_hit = datetime('now')`,
		normalized, slug, string(data),
	)
}

// GetPromotionCandidates returns cache entries with high hit counts
// that should be considered for promotion to the quick-match map.
func (c *RecCache) GetPromotionCandidates(minHits int) []string {
	rows, err := c.db.Query(
		`SELECT normalized_input, bundle_slug, hit_count 
         FROM recommendation_cache 
         WHERE hit_count >= ? 
         ORDER BY hit_count DESC`,
		minHits,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var candidates []string
	for rows.Next() {
		var input, slug string
		var hits int
		rows.Scan(&input, &slug, &hits)
		candidates = append(candidates,
			fmt.Sprintf("    %q: %q, // %d hits", input, slug, hits))
	}
	return candidates
}

// Stats returns cache statistics.
func (c *RecCache) Stats() (total int, totalHits int64) {
	c.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(hit_count),0) FROM recommendation_cache").Scan(&total, &totalHits)
	return
}

// personalize swaps in a business name without an LLM call.
func personalize(base *RecommendResult, businessName string) *RecommendResult {
	if businessName == "" {
		return base
	}

	// Deep copy to avoid mutating the cache
	data, _ := json.Marshal(base)
	var result RecommendResult
	json.Unmarshal(data, &result)

	// If the result title is generic, prepend business name
	if businessName != "" && !strings.Contains(result.Title, businessName) {
		result.Title = businessName + " — " + result.Title
	}

	return &result
}

// logCacheStats logs cache statistics on startup.
func logCacheStats(c *RecCache) {
	total, hits := c.Stats()
	log.Printf("[rec-cache] %d cached entries, %d total hits", total, hits)
}
