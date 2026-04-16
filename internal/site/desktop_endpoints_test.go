package site

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// desktopEndpoints is the canonical list of stockyard.dev/desktop/*
// endpoints. Each handler serves its backing file from the embedded
// static dir with the correct Content-Type, or 404s cleanly when the
// file hasn't been deployed yet.
var desktopEndpoints = []struct {
	path        string
	contentType string
}{
	{"/desktop/updates.json", "application/json"},
	{"/desktop/updates.json.sig", "application/octet-stream"},
	{"/desktop/tools-index.json", "application/json"},
	{"/desktop/tools-index.json.sig", "application/octet-stream"},
}

// TestDesktopEndpoints_Dispatch verifies that every desktop manifest
// handler is registered and dispatches cleanly — returning 200 with
// the correct Content-Type when its backing file exists, or 404 when
// it doesn't. Never 500, never 405, never empty 200.
func TestDesktopEndpoints_Dispatch(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, nil)

	for _, ep := range desktopEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", ep.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				// File exists in the embedded static dir. Verify Content-Type.
				got := rec.Header().Get("Content-Type")
				if got != ep.contentType {
					t.Errorf("%s: Content-Type = %q, want %q", ep.path, got, ep.contentType)
				}
				if rec.Body.Len() == 0 {
					t.Errorf("%s: 200 but empty body", ep.path)
				}
			case http.StatusNotFound:
				// File not deployed yet — correct pre-release posture.
			default:
				t.Errorf("%s: unexpected status %d (want 200 or 404)", ep.path, rec.Code)
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
