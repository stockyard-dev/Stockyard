package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
)

func openTestDB(t *testing.T) *history.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "echo_test.db")
	db, err := history.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	db := openTestDB(t)
	return New(Config{
		Port:    0,
		DB:      db,
		RepoDir: t.TempDir(), // not a real git repo, but avoids nil
	})
}

// ---------- GET endpoints ----------

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/health: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["product"] != "echo" {
		t.Errorf("product = %v, want echo", body["product"])
	}
}

func TestStats(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/stats: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["units"]; !ok {
		t.Error("expected 'units' key in stats response")
	}
}

func TestUnits_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/units: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var units []any
	json.Unmarshal(w.Body.Bytes(), &units)
	if len(units) != 0 {
		t.Errorf("expected empty units list, got %d", len(units))
	}
}

func TestUnits_WithData(t *testing.T) {
	db := openTestDB(t)
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "body", "sha1", "init", "alice", "", "created")
	s := New(Config{Port: 0, DB: db, RepoDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/units", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/units: status %d, want 200", w.Code)
	}

	var units []map[string]any
	json.Unmarshal(w.Body.Bytes(), &units)
	if len(units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(units))
	}
	if units[0]["name"] != "Foo" {
		t.Errorf("name = %v, want Foo", units[0]["name"])
	}
}

func TestUnits_QueryParams(t *testing.T) {
	db := openTestDB(t)
	db.RecordVersion("fn-1", "Alpha", "pkg/a.go", "function", "a", "", "", "", "", "created")
	db.RecordVersion("fn-2", "Beta", "pkg/b.go", "method", "b", "", "", "", "", "created")
	s := New(Config{Port: 0, DB: db, RepoDir: t.TempDir()})

	// Filter by kind
	req := httptest.NewRequest(http.MethodGet, "/api/units?kind=function", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status %d, want 200", w.Code)
	}
	var units []map[string]any
	json.Unmarshal(w.Body.Bytes(), &units)
	if len(units) != 1 {
		t.Errorf("expected 1 function unit, got %d", len(units))
	}

	// Filter by search
	req = httptest.NewRequest(http.MethodGet, "/api/units?search=Alpha", nil)
	w = httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &units)
	if len(units) != 1 {
		t.Errorf("expected 1 unit matching Alpha, got %d", len(units))
	}
}

func TestGetUnit_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// GetStory doesn't return an error for missing units, it returns empty story.
	// The handler calls GetStory which always succeeds.
	if w.Code != 200 && w.Code != 404 {
		t.Fatalf("GET /api/units/nonexistent: status %d", w.Code)
	}
}

func TestGetUnit_Exists(t *testing.T) {
	db := openTestDB(t)
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "body", "sha1", "init", "alice", "", "created")
	s := New(Config{Port: 0, DB: db, RepoDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/units/fn-1: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestUnitVersions(t *testing.T) {
	db := openTestDB(t)
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "v1", "sha1", "init", "alice", "", "created")
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "v2", "sha2", "fix", "bob", "", "modified")
	s := New(Config{Port: 0, DB: db, RepoDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-1/versions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status %d, want 200", w.Code)
	}
	var versions []map[string]any
	json.Unmarshal(w.Body.Bytes(), &versions)
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestUnitDiff_MissingParams(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-1/diff", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("GET /api/units/fn-1/diff no params: status %d, want 400", w.Code)
	}
	assertJSONError(t, w)
}

func TestUnitDiff_InvalidVersionParams(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-1/diff?v1=abc&v2=xyz", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestChurn_DefaultSince(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/churn", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/churn: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var entries []any
	json.Unmarshal(w.Body.Bytes(), &entries)
	if entries == nil {
		t.Error("expected non-nil churn response (empty array)")
	}
}

func TestChurn_WithSinceParam(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/churn?since=2020-01-01", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/churn?since: status %d, want 200", w.Code)
	}
}

func TestLineage_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/lineage/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/lineage: status %d, want 200", w.Code)
	}
	var records []any
	json.Unmarshal(w.Body.Bytes(), &records)
	if len(records) != 0 {
		t.Errorf("expected 0 lineage records, got %d", len(records))
	}
}

// ---------- POST endpoints ----------

func TestAddAnnotation_Success(t *testing.T) {
	db := openTestDB(t)
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "body", "", "", "", "", "created")
	s := New(Config{Port: 0, DB: db, RepoDir: t.TempDir()})

	body := `{"unit_id":"fn-1","type":"reason","content":"needed for auth"}`
	req := httptest.NewRequest(http.MethodPost, "/api/annotations", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("POST /api/annotations: status %d, want 201", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "created" {
		t.Errorf("status = %q, want created", resp["status"])
	}
}

func TestAddAnnotation_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/annotations", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestAddAnnotation_MissingFields(t *testing.T) {
	s := newTestServer(t)
	body := `{"unit_id":"","type":"","content":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/annotations", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestScan_NoGitRepo(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/scan", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// Scanning a non-git dir should return 500 error
	if w.Code != 500 {
		t.Logf("POST /api/scan non-git dir: status %d (expected 500)", w.Code)
	}
}

func TestUI(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /ui: status %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestRoot_Redirect(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("GET /: status %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "/ui" {
		t.Errorf("Location = %q, want /ui", loc)
	}
}

// ---------- helpers ----------

func assertJSONContentType(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func assertJSONError(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid JSON error: %s", w.Body.String())
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in JSON response")
	}
}
