package engine

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func setupPlaygroundTest(t *testing.T) (*http.ServeMux, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerPlaygroundRoutes(mux, db)
	return mux, db
}

func TestPlaygroundShareRoundtrip(t *testing.T) {
	mux, db := setupPlaygroundTest(t)
	defer db.Close()

	// Create a share
	body := `{"messages":[{"role":"user","content":"hello"}],"model":"gpt-4o"}`
	req := httptest.NewRequest("POST", "/api/playground/share", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("create: status %d, body: %s", w.Code, w.Body.String())
	}

	var created struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("create: unmarshal: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create: empty ID")
	}
	if created.URL == "" {
		t.Fatal("create: empty URL")
	}

	// Retrieve the share
	req2 := httptest.NewRequest("GET", "/api/playground/share/"+created.ID, nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("get: status %d, body: %s", w2.Code, w2.Body.String())
	}

	var share PlaygroundShare
	if err := json.Unmarshal(w2.Body.Bytes(), &share); err != nil {
		t.Fatalf("get: unmarshal: %v", err)
	}
	if share.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", share.Model)
	}
}

func TestPlaygroundShareMissingMessages(t *testing.T) {
	mux, db := setupPlaygroundTest(t)
	defer db.Close()

	body := `{"model":"gpt-4o"}`
	req := httptest.NewRequest("POST", "/api/playground/share", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPlaygroundShareNotFound(t *testing.T) {
	mux, db := setupPlaygroundTest(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/api/playground/share/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
