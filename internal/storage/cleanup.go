package storage

import (
	"log"
	"time"
)

// CleanupOldData deletes request logs and cache entries older than retentionDays.
func (db *DB) CleanupOldData(retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format(time.RFC3339)

	// Delete old request logs
	result, err := db.conn.Exec("DELETE FROM requests WHERE timestamp < ?", cutoff)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("cleanup: deleted %d old request logs", rows)
	}

	// Delete expired cache entries
	result, err = db.conn.Exec("DELETE FROM cache_entries WHERE expires_at < ?",
		time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("cleanup: deleted %d expired cache entries", rows)
	}

	return nil
}

// StartCleanupLoop runs cleanup periodically in the background.
func (db *DB) StartCleanupLoop(retentionDays int, interval time.Duration) {
	if retentionDays <= 0 {
		return
	}
	if interval == 0 {
		interval = 1 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := db.CleanupOldData(retentionDays); err != nil {
				log.Printf("cleanup error: %v", err)
			}
		}
	}()
}
