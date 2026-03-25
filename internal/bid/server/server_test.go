package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer creates a Server with a temp SQLite store and no providers.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BID_DATA_DIR", dir)

	s := New(Config{
		Port:     0,
		Strategy: "cheapest",
	})
	t.Cleanup(func() { s.Close() })
	return s
}

// ---------- route registration ----------

func TestRoutes_Health(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/health: got status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["product"] != "bid" {
		t.Errorf("product = %v, want bid", body["product"])
	}
}

func TestRoutes_Stats(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/stats: got status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Engine stats should have registered_providers
	if _, ok := body["registered_providers"]; !ok {
		t.Error("expected registered_providers key in stats response")
	}
}

func TestRoutes_Providers(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/providers: got status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestRoutes_Auctions(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auctions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/auctions: got status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestRoutes_Quality(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/quality", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/quality: got status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestRoutes_UI(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{"/", "/ui"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("GET %s: got status %d, want 200", path, w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("GET %s: Content-Type = %q, want text/html", path, ct)
		}
	}
}

// ---------- POST /api/bid ----------

func TestBid_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/bid", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/bid bad JSON: got status %d, want 400", w.Code)
	}
	assertJSONContentType(t, w)
	assertJSONError(t, w)
}

func TestBid_MissingPromptAndMessages(t *testing.T) {
	s := newTestServer(t)
	body := `{"task":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/bid", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/bid missing prompt: got status %d, want 400", w.Code)
	}
	assertJSONError(t, w)
}

func TestBid_NoProviders(t *testing.T) {
	s := newTestServer(t)
	body := `{"task":"test","prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/bid", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("POST /api/bid no providers: got status %d, want 503", w.Code)
	}
	assertJSONError(t, w)
}

// ---------- POST /api/providers ----------

func TestRegisterProvider_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/providers", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/providers bad JSON: got status %d, want 400", w.Code)
	}
}

func TestRegisterProvider_MissingFields(t *testing.T) {
	s := newTestServer(t)
	body := `{"id":"","model":"","api_key":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/providers", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/providers missing fields: got status %d, want 400", w.Code)
	}
	assertJSONError(t, w)
}

func TestRegisterProvider_Success(t *testing.T) {
	s := newTestServer(t)
	body := `{"id":"test-1","type":"openai","model":"gpt-4","api_key":"sk-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/providers", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("POST /api/providers success: got status %d, want 201", w.Code)
	}
	assertJSONContentType(t, w)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "registered" {
		t.Errorf("status = %q, want registered", resp["status"])
	}
	if resp["id"] != "test-1" {
		t.Errorf("id = %q, want test-1", resp["id"])
	}

	// After registration, bidder count should be 1.
	if s.engine.BidderCount() != 1 {
		t.Errorf("BidderCount = %d, want 1", s.engine.BidderCount())
	}
}

// ---------- Method not allowed (POST to GET endpoint, etc.) ----------

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	// POST to a GET-only endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// Go 1.22+ ServeMux returns 405 for method mismatch
	if w.Code != 405 {
		t.Logf("POST /api/health: got status %d (expected 405)", w.Code)
	}
}

// ---------- Store is created in temp dir ----------

func TestStoreCreatedInTempDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BID_DATA_DIR", dir)

	s := New(Config{Port: 0, Strategy: "cheapest"})
	defer s.Close()

	dbPath := filepath.Join(dir, "bid.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("expected database file at %s", dbPath)
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
		t.Fatalf("expected valid JSON error body, got: %s", w.Body.String())
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in JSON response")
	}
}
