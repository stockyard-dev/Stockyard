package storage

// CacheStats holds aggregate cache statistics.
type CacheStats struct {
	Entries   int     `json:"entries"`
	HitRate   float64 `json:"hit_rate"`
	SizeBytes int64   `json:"size_bytes"`
	SavingsUSD float64 `json:"savings_usd"`
}

// GetCacheStats returns aggregate cache statistics from the database.
func (db *DB) GetCacheStats() (*CacheStats, error) {
	var stats CacheStats
	err := db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(hits), 0), COALESCE(SUM(cost_saved * hits), 0)
		FROM cache_entries
	`).Scan(&stats.Entries, &stats.SizeBytes, &stats.SavingsUSD)
	if err != nil {
		return &CacheStats{}, nil
	}
	return &stats, nil
}

// ClearCache removes all cache entries.
func (db *DB) ClearCache() error {
	_, err := db.conn.Exec("DELETE FROM cache_entries")
	return err
}
