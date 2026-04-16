package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestBackupList_HappyPath: create an account with 3 backups, list
// them, verify they come back newest-first with correct fields.
func TestBackupList_HappyPath(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "list@example.com"
	acctObj, err := db.CreateCloudAccount(email, "cloud-multi", "cus_L", "sub_L")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteID, err := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "default")
	if err != nil {
		t.Fatalf("resolve site: %v", err)
	}

	// Insert three blobs spaced by explicit timestamps so ordering is
	// deterministic.
	for i := 1; i <= 3; i++ {
		if _, err := db.conn.Exec(`
			INSERT INTO cloud_backup_blobs
			    (account_id, site_id, blob_key, size_bytes, sha256_hex, client_version, uploaded_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now', ?))
		`, acctObj.ID, siteID, fmt.Sprintf("blob-%d", i), 1024*i, fmt.Sprintf("sha%d", i), "v0.2.0",
			fmt.Sprintf("+%d seconds", i)); err != nil {
			t.Fatalf("insert blob %d: %v", i, err)
		}
	}

	// Seed a bearer license so requireSession resolves the account.
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")

	r := httptest.NewRequest("GET", "/api/cloud/desktop/backups", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	w := httptest.NewRecorder()
	svc.HandleBackupList(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Backups    []backupListItem `json:"backups"`
		HasMore    bool             `json:"has_more"`
		NextBefore int64            `json:"next_before"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, w.Body.String())
	}
	if len(resp.Backups) != 3 {
		t.Fatalf("expected 3 backups, got %d", len(resp.Backups))
	}
	// Newest first: blob-3 was inserted with the latest offset.
	if resp.Backups[0].Sha256 != "sha3" {
		t.Fatalf("expected sha3 first, got %s", resp.Backups[0].Sha256)
	}
	if resp.Backups[2].Sha256 != "sha1" {
		t.Fatalf("expected sha1 last, got %s", resp.Backups[2].Sha256)
	}
	if resp.HasMore {
		t.Fatalf("expected has_more=false with only 3 blobs, got true")
	}
	// blob_key must NOT leak to the client.
	raw := w.Body.String()
	for i := 1; i <= 3; i++ {
		if strings.Contains(raw, fmt.Sprintf("blob-%d", i)) {
			t.Fatalf("blob_key leaked in list response: %s", raw)
		}
	}
}

// TestBackupList_Pagination: 25 blobs with default limit=20 returns
// 20 items and has_more=true with a usable next_before cursor.
func TestBackupList_Pagination(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "paginate@example.com"
	acctObj, err := db.CreateCloudAccount(email, "cloud-single", "cus_P", "sub_P")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-single"}, "default")

	for i := 1; i <= 25; i++ {
		if _, err := db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex)
			VALUES (?, ?, ?, ?, ?)
		`, acctObj.ID, siteID, fmt.Sprintf("k%d", i), 1000, fmt.Sprintf("s%d", i)); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-single")

	// First page.
	r := httptest.NewRequest("GET", "/api/cloud/desktop/backups", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	w := httptest.NewRecorder()
	svc.HandleBackupList(w, r)

	var page1 struct {
		Backups    []backupListItem `json:"backups"`
		HasMore    bool             `json:"has_more"`
		NextBefore int64            `json:"next_before"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page1.Backups) != 20 {
		t.Fatalf("expected 20 on first page, got %d", len(page1.Backups))
	}
	if !page1.HasMore {
		t.Fatalf("expected has_more=true with 25 total")
	}
	if page1.NextBefore <= 0 {
		t.Fatalf("expected positive next_before, got %d", page1.NextBefore)
	}

	// Second page using next_before cursor.
	r2 := httptest.NewRequest("GET",
		fmt.Sprintf("/api/cloud/desktop/backups?before=%d", page1.NextBefore), nil)
	r2.Header.Set("Authorization", "Bearer "+licenseKey)
	w2 := httptest.NewRecorder()
	svc.HandleBackupList(w2, r2)
	var page2 struct {
		Backups []backupListItem `json:"backups"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &page2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page2.Backups) != 5 {
		t.Fatalf("expected 5 on second page, got %d", len(page2.Backups))
	}
	if page2.HasMore {
		t.Fatalf("expected has_more=false on last page")
	}
}

// TestBackupByID_OwnershipEnforced: account A cannot download account
// B's backup by guessing its ID. Must 404, not 403 — no enumeration
// leakage.
func TestBackupByID_OwnershipEnforced(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	aAcct, _ := db.CreateCloudAccount("a@example.com", "cloud-single", "cus_a", "sub_a")
	bAcct, _ := db.CreateCloudAccount("b@example.com", "cloud-single", "cus_b", "sub_b")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	siteA, _ := svc.resolveOrCreateSite(&CloudAccount{ID: aAcct.ID, Tier: "cloud-single"}, "default")
	siteB, _ := svc.resolveOrCreateSite(&CloudAccount{ID: bAcct.ID, Tier: "cloud-single"}, "default")

	// A uploads a blob.
	var aBlobID int64
	res, err := db.conn.Exec(`
		INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex)
		VALUES (?, ?, ?, ?, ?)
	`, aAcct.ID, siteA, "a-blob", 500, "sha-a")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	aBlobID, _ = res.LastInsertId()
	_ = siteB

	// B authenticates and tries to fetch A's blob.
	bKey := seedActiveCloudLicense(t, db, "b@example.com", "cloud-single")

	r := httptest.NewRequest("GET", fmt.Sprintf("/api/cloud/desktop/backup/%d", aBlobID), nil)
	r.Header.Set("Authorization", "Bearer "+bKey)
	r.SetPathValue("id", fmt.Sprintf("%d", aBlobID))
	w := httptest.NewRecorder()
	svc.HandleBackupByID(w, r)

	if w.Code != 404 {
		t.Fatalf("expected 404 for cross-account fetch, got %d: %s", w.Code, w.Body.String())
	}
	// The 404 must look identical to a genuinely-missing blob — no
	// "this belongs to another account" leak.
	if strings.Contains(strings.ToLower(w.Body.String()), "another account") ||
		strings.Contains(strings.ToLower(w.Body.String()), "forbidden") ||
		strings.Contains(strings.ToLower(w.Body.String()), "permission") {
		t.Fatalf("404 body leaks ownership info: %s", w.Body.String())
	}
}

// TestBackupByID_InvalidID: non-numeric id → 400.
func TestBackupByID_InvalidID(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "bad@example.com"
	db.CreateCloudAccount(email, "cloud-single", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-single")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/api/cloud/desktop/backup/not-a-number", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	r.SetPathValue("id", "not-a-number")
	w := httptest.NewRecorder()
	svc.HandleBackupByID(w, r)

	if w.Code != 400 {
		t.Fatalf("expected 400 for non-numeric id, got %d", w.Code)
	}
}

// TestBackupList_EmptyAccount: new account, no uploads, returns empty
// list (not an error).
func TestBackupList_EmptyAccount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "empty@example.com"
	db.CreateCloudAccount(email, "cloud-single", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-single")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	// Must create the site first — list joins on cloud_sites.
	acct, _ := db.GetCloudAccountByEmail(email)
	svc.resolveOrCreateSite(acct, "default")

	r := httptest.NewRequest("GET", "/api/cloud/desktop/backups", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	w := httptest.NewRecorder()
	svc.HandleBackupList(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Backups []backupListItem `json:"backups"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Backups) != 0 {
		t.Fatalf("expected 0 backups, got %d", len(resp.Backups))
	}
}

// --- helpers --------------------------------------------------------

// noopBlobs is a BlobStore stub for tests that don't actually exercise
// blob IO. Get returns ErrBlobNotFound so HandleBackupByID's 500 path
// would be hit only if we accidentally wired a test to call it.
type noopBlobs struct{}

func (noopBlobs) Put(key string, r io.Reader) (int64, error) {
	return 0, fmt.Errorf("noopBlobs.Put not implemented")
}
func (noopBlobs) Get(key string) (io.ReadCloser, error) {
	return nil, ErrBlobNotFound
}
func (noopBlobs) Delete(key string) error { return nil }

// seedActiveCloudLicense inserts a matching license row so
// requireSession's bearer-auth path resolves to the account.
func seedActiveCloudLicense(t *testing.T, db *SqliteDB, email, tier string) string {
	t.Helper()
	key := fmt.Sprintf("SY-test-%s-%s.sig", tier, email)
	rec := &LicenseRecord{
		StripeCustomerID: "cus_" + email,
		Product:          "stockyard-desktop",
		Tier:             tier,
		LicenseKey:       key,
		Status:           "active",
		Email:            email,
		ExpiresAt:        time.Now().Add(10 * 365 * 24 * time.Hour),
	}
	if err := db.CreateLicense(rec); err != nil {
		t.Fatalf("seed license: %v", err)
	}
	return key
}

// TestRoutePrecedence_LatestVsID: Go 1.22+ ServeMux must prefer the
// literal /backup/latest over the wildcard /backup/{id}. If precedence
// breaks, "latest" gets parsed as an int64, fails, and the list page
// UI would silently break.
//
// This exercises the real mux wiring in server.go, not the handler
// directly, because the bug we care about is precisely a routing bug.
func TestRoutePrecedence_LatestVsID(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "route@example.com"
	db.CreateCloudAccount(email, "cloud-single", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-single")

	t.Setenv("STOCKYARD_CLOUD_ENABLED", "1")
	srv := NewServer(ServerConfig{Port: 0, AdminKey: ""}, db, nil, nil, &LogMailer{})

	// Make a site + one backup so /latest has something to return.
	svc := srv.desktopCloud
	acct, _ := db.GetCloudAccountByEmail(email)
	siteID, _ := svc.resolveOrCreateSite(acct, "default")
	db.conn.Exec(`
		INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex)
		VALUES (?, ?, ?, ?, ?)
	`, acct.ID, siteID, "some-key", 100, "somesha")

	// Hit /latest via the real mux. We expect the handler to try
	// fetching the blob (will fail — no blob store wired — which is
	// fine; we only care that it's NOT routed to HandleBackupByID).
	r := httptest.NewRequest("GET", "/api/cloud/desktop/backup/latest", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, r)

	// HandleBackupByID on "latest" would 400 "invalid backup id".
	// HandleBackupLatest without blob store wired returns 503.
	// So 400 = routing bug, 503 or 500 = routing fine.
	if w.Code == 400 && strings.Contains(w.Body.String(), "invalid backup id") {
		t.Fatalf("routing bug: /backup/latest was matched by /backup/{id} — got %s",
			w.Body.String())
	}
}
