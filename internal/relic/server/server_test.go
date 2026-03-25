package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/relic/store"
	"os"
	"path/filepath"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(Config{Store: db})
}

func TestHealth(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestCertifyAndGet(t *testing.T) {
	s := testServer(t)
	body := `{"trace_id":"t1","content":"hello","model":"gpt-4","provider":"openai","prompt":"test"}`
	req := httptest.NewRequest("POST", "/api/certify", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("certify: got %d, want 201: %s", w.Code, w.Body.String())
	}
}

var _ = os.TempDir // silence import
