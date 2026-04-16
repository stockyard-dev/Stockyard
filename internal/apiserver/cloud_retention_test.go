package apiserver

import (
	"fmt"
	"testing"
)

// TestRetention_UnderCount: account with fewer than N blobs, all
// recent — prune is a no-op.
func TestRetention_UnderCount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	acctObj, _ := db.CreateCloudAccount("u1@example.com", "cloud-multi", "c", "s")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "default")

	// Insert 5 blobs, all uploaded "now"
	for i := 0; i < 5; i++ {
		if _, err := db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex)
			VALUES (?, ?, ?, ?, ?)
		`, acctObj.ID, siteID, fmt.Sprintf("k%d", i), 1, "s"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	pruned, err := svc.pruneOldBackups(acctObj.ID, siteID)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("expected 0 pruned, got %d", pruned)
	}
	// Verify all 5 still exist.
	var count int
	db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_backup_blobs WHERE account_id=? AND site_id=?`,
		acctObj.ID, siteID).Scan(&count)
	if count != 5 {
		t.Fatalf("want 5 blobs remaining, got %d", count)
	}
}

// TestRetention_CountTriggersButProtectedByDays: exceed count but all
// blobs are recent — nothing pruned because the day floor protects
// them. This is the hybrid-rule payoff case.
func TestRetention_CountTriggersButProtectedByDays(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", "3")
	t.Setenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS", "7")

	acctObj, _ := db.CreateCloudAccount("u2@example.com", "cloud-multi", "c", "s")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "default")

	// 10 blobs, all uploaded today — count limit says keep 3, but
	// day floor says keep everything < 7 days old, so nothing
	// should be pruned.
	for i := 0; i < 10; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', ?))
		`, acctObj.ID, siteID, fmt.Sprintf("k%d", i), 1, "s", fmt.Sprintf("-%d seconds", i))
	}

	pruned, err := svc.pruneOldBackups(acctObj.ID, siteID)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("day floor should have protected all 10 recent blobs; got %d pruned", pruned)
	}
}

// TestRetention_DaysTriggersButProtectedByCount: blobs older than
// day floor but fewer than count — nothing pruned. Rare customer
// (one backup per month) whose count is tiny but ages are high.
func TestRetention_DaysTriggersButProtectedByCount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", "30")
	t.Setenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS", "7")

	acctObj, _ := db.CreateCloudAccount("u3@example.com", "cloud-multi", "c", "s")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "default")

	// 5 blobs, all 30 days old. Count limit is 30 so all 5 are
	// within count; day floor is 7 so all 5 are outside day window.
	// Intersection (outside count AND outside days) is empty =>
	// nothing pruned.
	for i := 0; i < 5; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
		`, acctObj.ID, siteID, fmt.Sprintf("k%d", i), 1, "s")
	}

	pruned, err := svc.pruneOldBackups(acctObj.ID, siteID)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("count ceiling should have protected all 5 old blobs; got %d pruned", pruned)
	}
}

// TestRetention_BothConditionsTrigger: lots of blobs, old ones outside
// day floor AND outside count limit — those get pruned. Recent blobs
// within day window stay regardless of count.
func TestRetention_BothConditionsTrigger(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", "3")
	t.Setenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS", "7")

	acctObj, _ := db.CreateCloudAccount("u4@example.com", "cloud-multi", "c", "s")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "default")

	// Setup: 3 recent + 5 old.
	// Recent (< 7 days): protected by day floor always.
	// Count limit is 3, so the top-3-by-uploaded_at are protected.
	//   The newest 3 will be the 3 recent ones (most recent
	//   uploaded_at wins), so the 5 old ones are outside both
	//   conditions. Expect 5 pruned.
	for i := 0; i < 5; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
		`, acctObj.ID, siteID, fmt.Sprintf("old-%d", i), 1, "s")
	}
	for i := 0; i < 3; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex)
			VALUES (?, ?, ?, ?, ?)
		`, acctObj.ID, siteID, fmt.Sprintf("new-%d", i), 1, "s")
	}

	pruned, err := svc.pruneOldBackups(acctObj.ID, siteID)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 5 {
		t.Fatalf("want 5 pruned (the old ones), got %d", pruned)
	}

	// Verify only the 3 recent ones remain.
	var remaining int
	db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_backup_blobs WHERE account_id=? AND site_id=?`,
		acctObj.ID, siteID).Scan(&remaining)
	if remaining != 3 {
		t.Fatalf("want 3 remaining, got %d", remaining)
	}
	// Verify the REMAINING ones are the new ones (by blob_key prefix).
	var newCount int
	db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_backup_blobs WHERE blob_key LIKE 'new-%'`).Scan(&newCount)
	if newCount != 3 {
		t.Fatalf("want 3 new-prefix blobs remaining, got %d", newCount)
	}
}

// TestRetention_ScopedToSite: pruning one site doesn't touch another
// site's blobs, even for the same account. Cloud Multi feature —
// Downtown's old backups must not get pruned when Uptown uploads.
func TestRetention_ScopedToSite(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", "2")
	t.Setenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS", "7")

	acctObj, _ := db.CreateCloudAccount("u5@example.com", "cloud-multi", "c", "s")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	downtownID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "downtown")
	uptownID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "uptown")

	// Downtown: 4 old blobs. Count=2, all >7d → 2 should be pruned.
	for i := 0; i < 4; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
		`, acctObj.ID, downtownID, fmt.Sprintf("dt-%d", i), 1, "s")
	}
	// Uptown: 4 old blobs too. Should NOT be touched when downtown prunes.
	for i := 0; i < 4; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
		`, acctObj.ID, uptownID, fmt.Sprintf("up-%d", i), 1, "s")
	}

	// Prune downtown only.
	pruned, err := svc.pruneOldBackups(acctObj.ID, downtownID)
	if err != nil {
		t.Fatalf("prune downtown: %v", err)
	}
	if pruned != 2 {
		t.Fatalf("downtown: want 2 pruned, got %d", pruned)
	}

	// Uptown must still have all 4.
	var uptownCount int
	db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_backup_blobs WHERE site_id = ?`, uptownID).Scan(&uptownCount)
	if uptownCount != 4 {
		t.Fatalf("uptown should be untouched (4), got %d", uptownCount)
	}
}

// TestRetention_ScopedToAccount: account A's prune doesn't touch
// account B's blobs. Defense against a bug where the WHERE clause
// loses the account_id filter.
func TestRetention_ScopedToAccount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", "1")
	t.Setenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS", "0")

	a, _ := db.CreateCloudAccount("a@example.com", "cloud-multi", "c", "s")
	b, _ := db.CreateCloudAccount("b@example.com", "cloud-multi", "c", "s")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteA, _ := svc.resolveOrCreateSite(&CloudAccount{ID: a.ID, Tier: "cloud-multi"}, "default")
	siteB, _ := svc.resolveOrCreateSite(&CloudAccount{ID: b.ID, Tier: "cloud-multi"}, "default")

	for i := 0; i < 3; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
		`, a.ID, siteA, fmt.Sprintf("a-%d", i), 1, "s")
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, uploaded_at)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
		`, b.ID, siteB, fmt.Sprintf("b-%d", i), 1, "s")
	}

	// Prune account A only. Count=1, days=0 → should prune A's oldest 2.
	pruned, err := svc.pruneOldBackups(a.ID, siteA)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 2 {
		t.Fatalf("want 2 pruned for A, got %d", pruned)
	}

	// B must have all 3.
	var bCount int
	db.conn.QueryRow(`SELECT COUNT(*) FROM cloud_backup_blobs WHERE account_id=?`, b.ID).Scan(&bCount)
	if bCount != 3 {
		t.Fatalf("B should be untouched (3), got %d", bCount)
	}
}

// TestRetention_DefaultsWhenEnvUnset: no env vars set → defaults
// (count=30, days=7) apply.
func TestRetention_DefaultsWhenEnvUnset(t *testing.T) {
	// Explicit unset in case parent test leaked.
	t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", "")
	t.Setenv("STOCKYARD_CLOUD_RETENTION_MIN_DAYS", "")

	count, days := retentionConfig()
	if count != retentionDefaultCount {
		t.Fatalf("want default count %d, got %d", retentionDefaultCount, count)
	}
	if days != retentionDefaultMinDays {
		t.Fatalf("want default days %d, got %d", retentionDefaultMinDays, days)
	}
}

// TestRetention_InvalidEnvIgnored: non-numeric and negative env vars
// fall back to defaults rather than crashing or using bad values.
func TestRetention_InvalidEnvIgnored(t *testing.T) {
	for _, bad := range []string{"not-a-number", "-5", "3.14", "∞"} {
		t.Setenv("STOCKYARD_CLOUD_RETENTION_COUNT", bad)
		count, _ := retentionConfig()
		if count != retentionDefaultCount {
			t.Errorf("bad count value %q should fall back to default, got %d", bad, count)
		}
	}
}
