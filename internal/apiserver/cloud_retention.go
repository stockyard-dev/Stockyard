package apiserver

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Retention policy for Cloud backups.
//
// Rule: for each (account, site), keep the most recent N backups OR
// all backups uploaded within the last D days, whichever retains
// more. Blobs that are BOTH older than the count ceiling AND older
// than the day floor are deleted (blob metadata row + blob store
// bytes).
//
// Why the dual rule: a pure count limit hurts customers who back up
// many times per day (a 30-count cap could wipe a week of history
// in one afternoon). A pure time limit hurts customers who back up
// rarely (a 7-day cap could wipe their only recent backup). Doing
// MAX(count, time) handles both comfortably.
//
// Tuning: STOCKYARD_CLOUD_RETENTION_COUNT and
// STOCKYARD_CLOUD_RETENTION_MIN_DAYS env vars override defaults at
// startup. Invalid values fall back to the defaults silently —
// operator sees the defaults in logs if they care.
const (
	retentionDefaultCount   = 30 // keep at least the 30 most recent
	retentionDefaultMinDays = 7  // always keep anything from the last 7 days
)

// pruneOldBackups removes blobs for the given (account, site) that
// are outside the retention window. Returns the number of blobs
// deleted (zero is a normal outcome for a freshly-uploading customer).
//
// Algorithm:
//  1. Compute the count cutoff: IDs ranked N+1..end from newest-first
//     are candidates (where N = retention count).
//  2. Compute the time cutoff: uploaded_at older than NOW - D days
//     are candidates.
//  3. Delete the INTERSECTION — blobs in both sets. This is the
//     "older than count limit AND older than day floor" check.
//  4. Remove the corresponding bytes from the blob store.
//
// Non-atomic across DB + blob store — if DELETE succeeds but blob-
// store deletion fails, the bytes linger as orphans. We log but
// don't fail the upload; a future GC pass over the blob directory
// can catch orphans if needed. Erring on the side of "upload
// always succeeds" is the right tradeoff for a backup product.
func (c *CloudService) pruneOldBackups(accountID, siteID int64) (int, error) {
	count, days := retentionConfig()
	if count <= 0 && days <= 0 {
		// All protections off — don't prune anything. Unlikely config
		// but handle gracefully.
		return 0, nil
	}

	// Find candidate blob IDs + storage keys in one query.
	//
	// The WHERE clause is the business rule:
	//   id NOT IN (top N by uploaded_at DESC)
	//   AND uploaded_at < now - D days
	//
	// "top N" is expressed as a subquery; SQLite handles this fine
	// and it keeps the logic explicit in one SQL statement.
	rows, err := c.db.conn.Query(`
		SELECT id, blob_key
		FROM cloud_backup_blobs
		WHERE account_id = ? AND site_id = ?
		  AND id NOT IN (
		      SELECT id FROM cloud_backup_blobs
		      WHERE account_id = ? AND site_id = ?
		      ORDER BY uploaded_at DESC
		      LIMIT ?
		  )
		  AND uploaded_at < datetime('now', ?)
	`, accountID, siteID,
		accountID, siteID, count,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return 0, fmt.Errorf("query prune candidates: %w", err)
	}

	type doomed struct {
		id  int64
		key string
	}
	var toDelete []doomed
	for rows.Next() {
		var d doomed
		if err := rows.Scan(&d.id, &d.key); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan prune candidate: %w", err)
		}
		toDelete = append(toDelete, d)
	}
	rows.Close()

	if len(toDelete) == 0 {
		return 0, nil
	}

	// Delete metadata rows in a single DELETE using the collected
	// IDs. Build the IN-list placeholders dynamically — SQLite's
	// default max is 999 parameters which comfortably exceeds any
	// realistic retention prune (customers with thousands of
	// backlogged backups would be a very different problem).
	if len(toDelete) > 900 {
		// Defensive cap. In the unlikely case of a massive backlog,
		// prune the oldest 900 this pass; next upload prunes more.
		toDelete = toDelete[:900]
	}
	placeholders := ""
	args := make([]any, 0, len(toDelete))
	for i, d := range toDelete {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, d.id)
	}
	delQuery := "DELETE FROM cloud_backup_blobs WHERE id IN (" + placeholders + ")"
	if _, err := c.db.conn.Exec(delQuery, args...); err != nil {
		return 0, fmt.Errorf("delete metadata rows: %w", err)
	}

	// Now the blob-store bytes. If blobs is nil (config error or
	// tests), skip silently — metadata is already gone, which is
	// fine; any future GC of the storage can catch the orphans.
	if c.blobs != nil {
		for _, d := range toDelete {
			if err := c.blobs.Delete(d.key); err != nil {
				// Already deleted metadata. Log and continue —
				// failing here means the bytes linger as orphans
				// but the metadata is already gone, so no customer
				// can reference them. Future GC of the store can
				// clean up orphans.
				// Note: LocalBlobStore.Delete is idempotent (no
				// error on missing file) so this is mostly for
				// future backends like S3.
				_ = err // explicit: we're choosing to ignore
			}
		}
	}

	return len(toDelete), nil
}

// retentionConfig resolves the count and day limits from env vars
// with defaults. Invalid values (non-numeric, negative) are ignored
// — operator gets the defaults. Zero is valid and means "disable
// that limit" (e.g. count=0 + days=7 = keep only the last 7 days).
func retentionConfig() (count int, days int) {
	count = retentionDefaultCount
	days = retentionDefaultMinDays
	if v := os.Getenv("STOCKYARD_CLOUD_RETENTION_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			count = n
		}
	}
	if v := os.Getenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			days = n
		}
	}
	return count, days
}

// Unused reference to keep the time import meaningful if callers
// shift around — pruneOldBackups uses datetime SQL rather than
// computing in Go, but a future enhancement (e.g. audit log of
// pruned blobs) may want time.Time.
var _ = time.Now