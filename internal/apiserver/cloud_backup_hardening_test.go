package apiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// inMemBlobs is a BlobStore stub that actually accepts writes, for
// tests that need uploads to succeed (rate limiting, key generation).
// It reads and discards the body (counting bytes) and stores nothing;
// we don't need to inspect blob contents for these tests.
type inMemBlobs struct {
	mu   sync.Mutex
	keys map[string]int64
}

func newInMemBlobs() *inMemBlobs {
	return &inMemBlobs{keys: map[string]int64{}}
}

func (b *inMemBlobs) Put(key string, r io.Reader) (int64, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return n, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.keys[key]; exists {
		// Match LocalBlobStore's overwrite semantics. We surface the
		// key-reuse fact via b.keys so tests can detect it.
		b.keys[key] = n
		return n, nil
	}
	b.keys[key] = n
	return n, nil
}
func (b *inMemBlobs) Get(key string) (io.ReadCloser, error) {
	return nil, ErrBlobNotFound
}
func (b *inMemBlobs) Delete(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.keys, key)
	return nil
}
func (b *inMemBlobs) KeyCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.keys)
}
func (b *inMemBlobs) Keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.keys))
	for k := range b.keys {
		out = append(out, k)
	}
	return out
}

// --- B1: pre-flight size check ---

// TestBackupUpload_PreflightSizeCheck: a client that sets
// Content-Length larger than maxBackupSize should get 413 WITHOUT
// any bytes being written to the blob store.
func TestBackupUpload_PreflightSizeCheck(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "preflight@example.com"
	acct, err := db.CreateCloudAccount(email, "cloud-single", "cus_P", "sub_P")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-single")
	blobs := newInMemBlobs()
	svc := NewCloudService(db, &LogMailer{}, blobs, "http://localhost", false)

	// Build a request with Content-Length larger than max.
	body := strings.NewReader("fake body doesn't matter")
	r := httptest.NewRequest("POST", "/api/cloud/desktop/backup", body)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	r.ContentLength = maxBackupSize + 1
	r.Header.Set("X-Site-Slug", "default")

	w := httptest.NewRecorder()
	svc.HandleBackupUpload(w, r)

	if w.Code != 413 {
		t.Fatalf("expected 413 (payload too large), got %d: %s", w.Code, w.Body.String())
	}
	if blobs.KeyCount() != 0 {
		t.Fatalf("expected no blobs written on preflight reject, got %d", blobs.KeyCount())
	}
	// Verify nothing made it into the DB either.
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM cloud_backup_blobs WHERE account_id = ?", acct.ID).Scan(&count)
	if count != 0 {
		t.Fatalf("expected no DB rows on preflight reject, got %d", count)
	}
}

// --- B3: blob key collision prevention ---

// TestBackupUpload_KeysUnique: 5 uploads in a tight loop (same account
// + site) should produce 5 distinct blob keys. Before the fix the key
// was acct-{N}-site-{N}-{nanos}.blob and same-nanosecond uploads
// produced identical keys; now there's an 8-byte random suffix.
func TestBackupUpload_KeysUnique(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "keys@example.com"
	if _, err := db.CreateCloudAccount(email, "cloud-multi", "cus_K", "sub_K"); err != nil {
		t.Fatalf("create account: %v", err)
	}
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")
	blobs := newInMemBlobs()
	svc := NewCloudService(db, &LogMailer{}, blobs, "http://localhost", false)

	for i := 0; i < 5; i++ {
		body := bytes.NewReader([]byte("payload"))
		r := httptest.NewRequest("POST", "/api/cloud/desktop/backup", body)
		r.Header.Set("Authorization", "Bearer "+licenseKey)
		r.Header.Set("X-Site-Slug", "default")
		r.ContentLength = 7

		w := httptest.NewRecorder()
		svc.HandleBackupUpload(w, r)

		if w.Code != 200 {
			t.Fatalf("upload %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	keys := blobs.Keys()
	if len(keys) != 5 {
		t.Fatalf("expected 5 unique keys, got %d: %v", len(keys), keys)
	}
	// Spot-check: all keys start with the expected prefix and contain
	// BOTH a nanosecond chunk and a random hex chunk (two numeric
	// sections separated by '-' after the site-id).
	for _, k := range keys {
		if !strings.HasPrefix(k, "acct-") || !strings.HasSuffix(k, ".blob") {
			t.Errorf("unexpected key shape: %s", k)
		}
		// acct-N-site-N-{nanos}-{hex}.blob has 5 dashes
		if n := strings.Count(k, "-"); n != 5 {
			t.Errorf("key %s has %d dashes, expected 5 (acct-N-site-N-nanos-rand)", k, n)
		}
	}
}

// --- B6: rate limiting ---

// TestBackupUpload_RateLimitHourly: exactly backupsPerHourLimit
// uploads succeed, the next one returns 429 with Retry-After set.
func TestBackupUpload_RateLimitHourly(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "rlh@example.com"
	if _, err := db.CreateCloudAccount(email, "cloud-multi", "cus_RH", "sub_RH"); err != nil {
		t.Fatalf("create account: %v", err)
	}
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")
	blobs := newInMemBlobs()
	svc := NewCloudService(db, &LogMailer{}, blobs, "http://localhost", false)

	doUpload := func() *httptest.ResponseRecorder {
		body := bytes.NewReader([]byte("x"))
		r := httptest.NewRequest("POST", "/api/cloud/desktop/backup", body)
		r.Header.Set("Authorization", "Bearer "+licenseKey)
		r.Header.Set("X-Site-Slug", "default")
		r.ContentLength = 1
		w := httptest.NewRecorder()
		svc.HandleBackupUpload(w, r)
		return w
	}

	// First N should succeed.
	for i := 0; i < backupsPerHourLimit; i++ {
		w := doUpload()
		if w.Code != 200 {
			t.Fatalf("upload %d/%d: expected 200, got %d: %s",
				i+1, backupsPerHourLimit, w.Code, w.Body.String())
		}
	}

	// The (N+1)th should 429.
	w := doUpload()
	if w.Code != 429 {
		t.Fatalf("upload %d: expected 429, got %d: %s",
			backupsPerHourLimit+1, w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Errorf("expected Retry-After header on 429, got empty")
	}

	// Verify the 429 response body mentions the limit.
	var rlResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &rlResp); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if !strings.Contains(rlResp["error"], "rate limit") {
		t.Errorf("expected error to mention rate limit, got: %s", rlResp["error"])
	}
	if !strings.Contains(rlResp["limit"], "per hour") {
		t.Errorf("expected limit to mention 'per hour', got: %s", rlResp["limit"])
	}
}

// TestBackupUpload_RateLimitIsolatedByAccount: account A hitting its
// limit does NOT affect account B.
func TestBackupUpload_RateLimitIsolatedByAccount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Account A maxes out the hourly limit.
	emailA := "a@example.com"
	if _, err := db.CreateCloudAccount(emailA, "cloud-multi", "cus_A", "sub_A"); err != nil {
		t.Fatalf("create A: %v", err)
	}
	keyA := seedActiveCloudLicense(t, db, emailA, "cloud-multi")

	// Account B with 0 uploads.
	emailB := "b@example.com"
	if _, err := db.CreateCloudAccount(emailB, "cloud-multi", "cus_B", "sub_B"); err != nil {
		t.Fatalf("create B: %v", err)
	}
	keyB := seedActiveCloudLicense(t, db, emailB, "cloud-multi")

	blobs := newInMemBlobs()
	svc := NewCloudService(db, &LogMailer{}, blobs, "http://localhost", false)

	upload := func(bearer string) int {
		body := bytes.NewReader([]byte("x"))
		r := httptest.NewRequest("POST", "/api/cloud/desktop/backup", body)
		r.Header.Set("Authorization", "Bearer "+bearer)
		r.Header.Set("X-Site-Slug", "default")
		r.ContentLength = 1
		w := httptest.NewRecorder()
		svc.HandleBackupUpload(w, r)
		return w.Code
	}

	for i := 0; i < backupsPerHourLimit; i++ {
		if code := upload(keyA); code != 200 {
			t.Fatalf("A upload %d: got %d", i+1, code)
		}
	}
	// A should be rate-limited now.
	if code := upload(keyA); code != 429 {
		t.Fatalf("A overflow: expected 429, got %d", code)
	}
	// But B should still be fine.
	if code := upload(keyB); code != 200 {
		t.Fatalf("B fresh upload: expected 200, got %d", code)
	}
}

// --- CountBackupsInWindow unit test ---

// TestCountBackupsInWindow: insert rows with controlled timestamps,
// verify the count matches for different window sizes.
func TestCountBackupsInWindow(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "cnt@example.com"
	acct, err := db.CreateCloudAccount(email, "cloud-multi", "cus_CNT", "sub_CNT")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := NewCloudService(db, &LogMailer{}, newInMemBlobs(), "http://localhost", false)
	siteID, err := svc.resolveOrCreateSite(&CloudAccount{ID: acct.ID, Tier: "cloud-multi"}, "default")
	if err != nil {
		t.Fatalf("resolve site: %v", err)
	}

	// Insert: one 30s old, one 2h old, one 2d old.
	cases := []string{"-30 seconds", "-2 hours", "-2 days"}
	for i, offset := range cases {
		if _, err := db.conn.Exec(`
			INSERT INTO cloud_backup_blobs
			    (account_id, site_id, blob_key, size_bytes, sha256_hex, client_version, uploaded_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now', ?))
		`, acct.ID, siteID, fmt.Sprintf("tkey-%d", i), 100, "sha", "v0", offset); err != nil {
			t.Fatalf("insert %s: %v", offset, err)
		}
	}

	// 1-minute window: only the 30s-old row.
	if n, err := db.CountBackupsInWindow(acct.ID, 60); err != nil || n != 1 {
		t.Errorf("1-minute window: n=%d err=%v, want n=1", n, err)
	}
	// 1-hour window: same (the 2h row is outside).
	if n, err := db.CountBackupsInWindow(acct.ID, 3600); err != nil || n != 1 {
		t.Errorf("1-hour window: n=%d err=%v, want n=1", n, err)
	}
	// 3-hour window: 30s + 2h = 2.
	if n, err := db.CountBackupsInWindow(acct.ID, 10800); err != nil || n != 2 {
		t.Errorf("3-hour window: n=%d err=%v, want n=2", n, err)
	}
	// 3-day window: all 3.
	if n, err := db.CountBackupsInWindow(acct.ID, 3*86400); err != nil || n != 3 {
		t.Errorf("3-day window: n=%d err=%v, want n=3", n, err)
	}
	// Zero-length window: 0.
	if n, err := db.CountBackupsInWindow(acct.ID, 0); err != nil || n != 0 {
		t.Errorf("0-window: n=%d err=%v, want n=0", n, err)
	}
}

// Sanity: limit constants are sensible values.
func TestRateLimitConstants(t *testing.T) {
	if backupsPerHourLimit <= 0 {
		t.Error("backupsPerHourLimit must be positive")
	}
	if backupsPerDayLimit <= backupsPerHourLimit {
		t.Error("daily limit should be higher than hourly limit")
	}
	// 10/hour × 24 hours = 240 — daily limit of 100 is tighter. Good.
	// Exercising this math here documents the intent.
	if backupsPerDayLimit >= backupsPerHourLimit*24 {
		t.Errorf("daily limit (%d) >= hourly × 24 (%d): the daily cap is not binding",
			backupsPerDayLimit, backupsPerHourLimit*24)
	}
}

// suppress unused-import complaints in case some helpers go unused
var _ = time.Now
