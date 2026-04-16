package apiserver

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSitesList_EmptyAccount: a fresh Cloud Multi account with no
// sites yet returns an empty list (and correct tier), not an error.
// This is the state a customer sees immediately after purchase,
// before the first backup is taken.
func TestSitesList_EmptyAccount(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "fresh@example.com"
	db.CreateCloudAccount(email, "cloud-multi", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/api/cloud/desktop/sites", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	w := httptest.NewRecorder()
	svc.HandleSitesList(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Sites []siteListItem `json:"sites"`
		Tier  string         `json:"tier"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Sites) != 0 {
		t.Fatalf("expected 0 sites, got %d", len(resp.Sites))
	}
	if resp.Tier != "cloud-multi" {
		t.Fatalf("expected tier cloud-multi, got %q", resp.Tier)
	}
}

// TestSitesList_WithBackups: sites with uploaded blobs report correct
// backup_count and non-empty last_backup. Sites without blobs still
// appear (LEFT JOIN behavior) with count=0 and empty last_backup.
func TestSitesList_WithBackups(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "multi@example.com"
	acctObj, _ := db.CreateCloudAccount(email, "cloud-multi", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)
	// Create two sites. downtown gets 2 backups, uptown gets 0.
	downtownID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "downtown")
	svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "uptown")

	for i := 1; i <= 2; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex)
			VALUES (?, ?, ?, ?, ?)
		`, acctObj.ID, downtownID, "k", 100, "s")
	}

	r := httptest.NewRequest("GET", "/api/cloud/desktop/sites", nil)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	w := httptest.NewRecorder()
	svc.HandleSitesList(w, r)

	var resp struct {
		Sites []siteListItem `json:"sites"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(resp.Sites))
	}
	var downtown, uptown siteListItem
	for _, s := range resp.Sites {
		switch s.Slug {
		case "downtown":
			downtown = s
		case "uptown":
			uptown = s
		}
	}
	if downtown.BackupCount != 2 {
		t.Fatalf("downtown: want 2 backups, got %d", downtown.BackupCount)
	}
	if uptown.BackupCount != 0 {
		t.Fatalf("uptown: want 0 backups, got %d", uptown.BackupCount)
	}
	if downtown.LastBackup == "" {
		t.Fatalf("downtown: last_backup should be non-empty")
	}
	if uptown.LastBackup != "" {
		t.Fatalf("uptown: last_backup should be empty, got %q", uptown.LastBackup)
	}
}

// TestSitesCreate_HappyPath: Cloud Multi account creates a new site,
// gets created=true. Second call with same slug returns created=false
// (idempotent).
func TestSitesCreate_HappyPath(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "create@example.com"
	db.CreateCloudAccount(email, "cloud-multi", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	// First call: created
	body := strings.NewReader(`{"slug":"downtown","display_name":"Downtown Studio"}`)
	r := httptest.NewRequest("POST", "/api/cloud/desktop/sites", body)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	svc.HandleSitesCreate(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp1 struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
		Created     bool   `json:"created"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp1)
	if !resp1.Created {
		t.Fatalf("expected created=true on first call")
	}
	if resp1.Slug != "downtown" || resp1.DisplayName != "Downtown Studio" {
		t.Fatalf("unexpected response: %+v", resp1)
	}

	// Second call: idempotent, same slug, returns existing row.
	body2 := strings.NewReader(`{"slug":"downtown","display_name":"Should Not Overwrite"}`)
	r2 := httptest.NewRequest("POST", "/api/cloud/desktop/sites", body2)
	r2.Header.Set("Authorization", "Bearer "+licenseKey)
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	svc.HandleSitesCreate(w2, r2)

	if w2.Code != 200 {
		t.Fatalf("expected 200 on second call, got %d", w2.Code)
	}
	var resp2 struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
		Created     bool   `json:"created"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Created {
		t.Fatalf("expected created=false on second call")
	}
	// Display name must NOT have been overwritten by the second call.
	if resp2.DisplayName != "Downtown Studio" {
		t.Fatalf("display_name got overwritten: %q", resp2.DisplayName)
	}
}

// TestSitesCreate_SingleTierLimit: a Cloud Single account can create
// exactly one site. The second attempt returns 403 with an upgrade
// message.
func TestSitesCreate_SingleTierLimit(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "single@example.com"
	db.CreateCloudAccount(email, "cloud-single", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-single")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	// First site: allowed
	body := strings.NewReader(`{"slug":"shop"}`)
	r := httptest.NewRequest("POST", "/api/cloud/desktop/sites", body)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	svc.HandleSitesCreate(w, r)
	if w.Code != 200 {
		t.Fatalf("first site: expected 200, got %d", w.Code)
	}

	// Second site: 403
	body2 := strings.NewReader(`{"slug":"warehouse"}`)
	r2 := httptest.NewRequest("POST", "/api/cloud/desktop/sites", body2)
	r2.Header.Set("Authorization", "Bearer "+licenseKey)
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	svc.HandleSitesCreate(w2, r2)
	if w2.Code != 403 {
		t.Fatalf("second site: expected 403, got %d: %s", w2.Code, w2.Body.String())
	}
	// Error body should mention upgrade.
	if !strings.Contains(strings.ToLower(w2.Body.String()), "upgrade") {
		t.Fatalf("403 body should suggest upgrade; got %s", w2.Body.String())
	}
}

// TestSitesCreate_InvalidSlug: reject slugs with uppercase, spaces,
// punctuation, too long, or empty.
func TestSitesCreate_InvalidSlug(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "slug@example.com"
	db.CreateCloudAccount(email, "cloud-multi", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	bad := []string{
		`{"slug":""}`,
		`{"slug":"Has Uppercase"}`,
		`{"slug":"has spaces"}`,
		`{"slug":"has/slash"}`,
		`{"slug":"has.dot"}`,
		`{"slug":"` + strings.Repeat("a", 41) + `"}`,
	}
	for _, body := range bad {
		r := httptest.NewRequest("POST", "/api/cloud/desktop/sites", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+licenseKey)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		svc.HandleSitesCreate(w, r)
		if w.Code != 400 {
			t.Errorf("body %s: expected 400, got %d (%s)", body, w.Code, w.Body.String())
		}
	}
}

// TestSitesCreate_NormalizesSlug: slug input is lowercased + trimmed
// before validation, so "  Downtown " with leading space + uppercase
// would land as "downtown" if it passed, but in this case it fails
// because uppercase is rejected in isValidSiteSlug and we normalize
// BEFORE that check. Actually: we lowercase THEN validate. Let's
// verify with whitespace-only normalization.
func TestSitesCreate_NormalizesSlug(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	email := "norm@example.com"
	db.CreateCloudAccount(email, "cloud-multi", "c", "s")
	licenseKey := seedActiveCloudLicense(t, db, email, "cloud-multi")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	// Leading/trailing whitespace + uppercase should be normalized
	// to "downtown" and succeed.
	body := strings.NewReader(`{"slug":"  DOWNTOWN  "}`)
	r := httptest.NewRequest("POST", "/api/cloud/desktop/sites", body)
	r.Header.Set("Authorization", "Bearer "+licenseKey)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	svc.HandleSitesCreate(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Slug string `json:"slug"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Slug != "downtown" {
		t.Fatalf("expected normalized 'downtown', got %q", resp.Slug)
	}
}
