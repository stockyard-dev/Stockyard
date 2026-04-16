package apiserver

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// Admin password used by these tests. We use t.Setenv so the env var
// is scoped to the test and restored on teardown. Never use a real
// or production-looking password here.
const testAdminPassword = "test-admin-pw"

// makeAdminReq builds a request with (or without) basic auth against
// testAdminPassword. Pass empty string for no-auth.
func makeAdminReq(t *testing.T, method, path, pw string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	if pw != "" {
		r.SetBasicAuth("admin", pw)
	}
	// Attach {id} path value if the route calls for it. httptest
	// doesn't parse path patterns, so tests that need the path
	// parameter must call SetPathValue themselves.
	return httptest.NewRecorder()
}

// TestAdmin_DisabledReturns404: with STOCKYARD_ADMIN_PASSWORD unset,
// every admin route must 404, regardless of whether basic-auth
// creds are presented. An attacker probing for /admin/ must get no
// signal that the endpoint exists.
func TestAdmin_DisabledReturns404(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", "")

	db, cleanup := newTestDB(t)
	defer cleanup()
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	for _, path := range []string{"/admin/", "/admin/json", "/admin/account/1"} {
		r := httptest.NewRequest("GET", path, nil)
		r.SetBasicAuth("admin", "anything") // even with creds, should 404
		w := httptest.NewRecorder()

		// adminGuard will check enabled-ness and 404 before touching
		// the wrapped handler. We drive it directly here rather than
		// through the mux to keep the test scoped.
		guard := svc.adminGuard(svc.HandleAdminIndex)
		guard(w, r)

		if w.Code != 404 {
			t.Errorf("%s: with empty env var, expected 404, got %d", path, w.Code)
		}
	}
}

// TestAdmin_UnauthorizedReturns401: password is set but the request
// doesn't present valid basic-auth, so the handler must return 401
// with a WWW-Authenticate header so a browser prompts for creds.
func TestAdmin_UnauthorizedReturns401(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	// No auth at all.
	r := httptest.NewRequest("GET", "/admin/", nil)
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminIndex)(w, r)
	if w.Code != 401 {
		t.Errorf("no-auth: expected 401, got %d", w.Code)
	}
	if wa := w.Header().Get("WWW-Authenticate"); !strings.Contains(wa, "Basic") {
		t.Errorf("expected WWW-Authenticate: Basic ..., got %q", wa)
	}

	// Wrong password.
	r2 := httptest.NewRequest("GET", "/admin/", nil)
	r2.SetBasicAuth("admin", "wrong-password")
	w2 := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminIndex)(w2, r2)
	if w2.Code != 401 {
		t.Errorf("wrong-pw: expected 401, got %d", w2.Code)
	}
}

// TestAdmin_AuthorizedIndexReturnsHTML: correct creds + populated DB
// returns 200 and HTML containing the seeded account email.
func TestAdmin_AuthorizedIndexReturnsHTML(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()

	// Seed three accounts with different tiers + statuses so we can
	// verify they all render.
	db.CreateCloudAccount("alice@example.com", "cloud-single", "cus_A", "sub_A")
	db.CreateCloudAccount("bob@example.com", "cloud-multi", "cus_B", "sub_B")
	db.CreateCloudAccount("carol@example.com", "cloud-single", "cus_C", "sub_C")

	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/admin/", nil)
	r.SetBasicAuth("admin", testAdminPassword)
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminIndex)(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("expected no-store cache, got %q", cc)
	}
	body := w.Body.String()
	for _, want := range []string{"alice@example.com", "bob@example.com", "carol@example.com",
		"cloud-single", "cloud-multi", "Stockyard Admin"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

// TestAdmin_EmptyStateRendersCleanly: no accounts in the DB still
// returns a 200 with an empty-state message (doesn't 500 on empty
// result set).
func TestAdmin_EmptyStateRendersCleanly(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/admin/", nil)
	r.SetBasicAuth("admin", testAdminPassword)
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminIndex)(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "No accounts yet") {
		t.Errorf("empty-state message not rendered")
	}
}

// TestAdmin_JSONEndpoint: /admin/json returns parseable JSON with
// the accounts array.
func TestAdmin_JSONEndpoint(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	db.CreateCloudAccount("json@example.com", "cloud-multi", "cus_J", "sub_J")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/admin/json", nil)
	r.SetBasicAuth("admin", testAdminPassword)
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminJSON)(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Accounts []struct {
			Email string `json:"Email"`
			Tier  string `json:"Tier"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if len(resp.Accounts) != 1 {
		t.Fatalf("want 1 account, got %d", len(resp.Accounts))
	}
	if resp.Accounts[0].Email != "json@example.com" {
		t.Errorf("unexpected email: %q", resp.Accounts[0].Email)
	}
}

// TestAdmin_AccountDrillDown: account detail page renders with the
// account's data, backup history, and sites. The specific account
// ID is passed via r.SetPathValue (httptest doesn't parse patterns).
func TestAdmin_AccountDrillDown(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	acctObj, _ := db.CreateCloudAccount("detail@example.com", "cloud-multi", "cus_D", "sub_D")
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	// Seed some data so the drill-down has something to show.
	siteID, _ := svc.resolveOrCreateSite(&CloudAccount{ID: acctObj.ID, Tier: "cloud-multi"}, "downtown")
	for i := 0; i < 3; i++ {
		db.conn.Exec(`
			INSERT INTO cloud_backup_blobs (account_id, site_id, blob_key, size_bytes, sha256_hex, client_version)
			VALUES (?, ?, ?, ?, ?, ?)
		`, acctObj.ID, siteID, "k", 1024*1024*5, "abc123def456def789", "desktop-v0.2.0")
	}

	r := httptest.NewRequest("GET", "/admin/account/"+itoa(acctObj.ID), nil)
	r.SetBasicAuth("admin", testAdminPassword)
	r.SetPathValue("id", itoa(acctObj.ID))
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminAccount)(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"detail@example.com",
		"cloud-multi",
		"downtown",
		"5.0 MB",          // bytes formatter output
		"abc123def456",    // shortened sha
		"desktop-v0.2.0",  // client version rendered
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

// TestAdmin_AccountDrillDown_NotFound: unknown account ID returns
// 404 without leaking whether the ID range exists.
func TestAdmin_AccountDrillDown_NotFound(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/admin/account/99999", nil)
	r.SetBasicAuth("admin", testAdminPassword)
	r.SetPathValue("id", "99999")
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminAccount)(w, r)

	if w.Code != 404 {
		t.Errorf("expected 404 for nonexistent account, got %d", w.Code)
	}
}

// TestAdmin_AccountDrillDown_BadID: malformed ID (non-numeric,
// negative, zero) returns 400 rather than 500 or accidentally
// matching something.
func TestAdmin_AccountDrillDown_BadID(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	for _, bad := range []string{"abc", "-1", "0"} {
		r := httptest.NewRequest("GET", "/admin/account/"+bad, nil)
		r.SetBasicAuth("admin", testAdminPassword)
		r.SetPathValue("id", bad)
		w := httptest.NewRecorder()
		svc.adminGuard(svc.HandleAdminAccount)(w, r)
		if w.Code != 400 {
			t.Errorf("id=%q: expected 400, got %d", bad, w.Code)
		}
	}
}

// TestAdmin_SecurityHeaders: admin responses must carry Cache-Control
// no-store and X-Robots-Tag noindex so customer data is never cached
// or indexed.
func TestAdmin_SecurityHeaders(t *testing.T) {
	t.Setenv("STOCKYARD_ADMIN_PASSWORD", testAdminPassword)

	db, cleanup := newTestDB(t)
	defer cleanup()
	svc := NewCloudService(db, &LogMailer{}, &noopBlobs{}, "http://localhost", false)

	r := httptest.NewRequest("GET", "/admin/", nil)
	r.SetBasicAuth("admin", testAdminPassword)
	w := httptest.NewRecorder()
	svc.adminGuard(svc.HandleAdminIndex)(w, r)

	if w.Code != 200 {
		t.Fatalf("setup sanity: got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Errorf("Cache-Control missing no-store: %q", w.Header().Get("Cache-Control"))
	}
	if !strings.Contains(w.Header().Get("X-Robots-Tag"), "noindex") {
		t.Errorf("X-Robots-Tag missing noindex: %q", w.Header().Get("X-Robots-Tag"))
	}
}

// itoa is a tiny helper to avoid pulling strconv into the test just
// for int64 -> string conversion.
func itoa(n int64) string {
	// Reuse strconv underneath. Keeping a helper keeps test call
	// sites readable.
	return strings.TrimSpace(stdintoa(n))
}

// stdintoa wraps strconv.FormatInt so itoa can read naturally in test
// call sites without an import.
func stdintoa(n int64) string {
	// This intentionally imports nothing new — the file already
	// imports "strings" and the apiserver package has strconv
	// available. We use a manual conversion to avoid adding an
	// import just for tests.
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
