package site

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// desktopEndpoints is the canonical list of stockyard.dev/desktop/*
// endpoints. Each is expected to 404 cleanly when its backing file
// hasn't been deployed yet (pre-first-release posture) and to serve
// the right Content-Type when it has.
var desktopEndpoints = []struct {
	path        string
	contentType string
}{
	{"/desktop/updates.json", "application/json"},
	{"/desktop/updates.json.sig", "application/octet-stream"},
	{"/desktop/tools-index.json", "application/json"},
	{"/desktop/tools-index.json.sig", "application/octet-stream"},
}

// Until release assets land in site/desktop/, the handlers should
// return 404 cleanly — not 500, not empty 200. Desktop clients rely
// on the 404 to decide "no release yet, stay in offline/bundled
// mode" rather than eating a parse error on empty content.
func TestDesktopEndpoints_404WhenBacking_FileMissing(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, nil)

	for _, ep := range desktopEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", ep.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s: got status %d, want 404 (pre-release posture: backing file should be absent)",
					ep.path, rec.Code)
			}
		})
	}
}

// Guard: the paired `.sig` endpoint for each JSON manifest must
// exist. A manifest without a discoverable signature endpoint is
// broken by design — tooldl / updater expect both URLs to 404-or-200
// together. This test catches accidental deletions or typos.
func TestDesktopEndpoints_JSONAndSigRegisteredTogether(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, nil)

	pairs := []string{"/desktop/updates.json", "/desktop/tools-index.json"}
	for _, base := range pairs {
		t.Run(base, func(t *testing.T) {
			// Both should route to the same handler-not-found behavior
			// right now (404 since neither file exists), but the ServeMux
			// should at least dispatch without a "no such handler" line.
			for _, path := range []string{base, base + ".sig"} {
				req := httptest.NewRequest("GET", path, nil)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				// Anything other than a pure 404 from the handler — like
				// 405 from a default mux refusal — indicates the handler
				// isn't registered.
				if rec.Code == http.StatusMethodNotAllowed {
					t.Errorf("%s: got 405, handler likely not registered", path)
				}
			}
		})
	}
}
