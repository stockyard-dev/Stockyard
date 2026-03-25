package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/spore/store"
)

func TestHealth(t *testing.T) {
	db, _ := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if db != nil { defer db.Close() }
	s := New(Config{Store: db})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/health", nil)
	s.ServeHTTP(w, r)
	if w.Code != 200 { t.Errorf("got %d", w.Code) }
}
