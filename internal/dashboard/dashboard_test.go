package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardServes(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, "costcap")

	req := httptest.NewRequest("GET", "/ui", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()

	// Verify product key was injected
	if !strings.Contains(body, `data-product="costcap"`) {
		t.Error("product key not injected into HTML")
	}

	// Verify product name was injected
	if !strings.Contains(body, "CostCap Dashboard") {
		t.Error("product name not injected into title")
	}

	// Verify it's valid HTML
	if !strings.HasPrefix(body, "<!DOCTYPE html>") {
		t.Error("response is not HTML")
	}

	// Verify Preact is loaded
	if !strings.Contains(body, "preact") {
		t.Error("Preact not found in HTML")
	}
}

func TestDashboardProducts(t *testing.T) {
	products := []struct {
		key  string
		name string
	}{
		{"costcap", "CostCap"},
		{"llmcache", "CacheLayer"},
		{"jsonguard", "StructuredShield"},
		{"routefall", "FallbackRouter"},
		{"rateshield", "RateShield"},
		{"promptreplay", "PromptReplay"},
		{"stockyard", "Stockyard"},
	}

	for _, tt := range products {
		t.Run(tt.key, func(t *testing.T) {
			mux := http.NewServeMux()
			Register(mux, tt.key)

			req := httptest.NewRequest("GET", "/ui", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d", w.Code)
			}
			body := w.Body.String()
			if !strings.Contains(body, `data-product="`+tt.key+`"`) {
				t.Errorf("product key %q not injected", tt.key)
			}
			if !strings.Contains(body, tt.name+" Dashboard") {
				t.Errorf("product name %q not in title", tt.name)
			}
		})
	}
}

func TestBroadcaster(t *testing.T) {
	b := NewBroadcaster()

	if b.ClientCount() != 0 {
		t.Errorf("client count = %d, want 0", b.ClientCount())
	}

	// Send with no clients should not panic
	b.Send(map[string]string{"type": "test"})
}
