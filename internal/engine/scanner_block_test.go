package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScannerBlockMiddleware(t *testing.T) {
	// Inner handler always returns 200 — middleware should short-circuit
	// before this runs for blocked paths.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := scannerBlockMiddleware(inner)

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		// Should be blocked (real scanner paths observed in production)
		{"dotenv root", "/.env", http.StatusForbidden},
		{"dotenv backup", "/.env.bak", http.StatusForbidden},
		{"dotenv production", "/.env.production", http.StatusForbidden},
		{"dotenv test", "/.env.test", http.StatusForbidden},
		{"dotenv nested backend", "/backend/.env", http.StatusForbidden},
		{"dotenv nested storage", "/storage/.env", http.StatusForbidden},
		{"phpinfo plain", "/phpinfo.php", http.StatusForbidden},
		{"phpinfo old", "/old_phpinfo.php", http.StatusForbidden},
		{"phpinfo nested", "/hosting/phpinfo.php", http.StatusForbidden},
		{"wp-admin install", "/wp-admin/install.php", http.StatusForbidden},
		{"wp-login", "/wp-login.php", http.StatusForbidden},
		{"debug.php", "/debug.php", http.StatusForbidden},
		{"php.php", "/php.php", http.StatusForbidden},

		// Should pass through (real site paths must NEVER be blocked)
		{"homepage", "/", http.StatusOK},
		{"pricing", "/pricing/", http.StatusOK},
		{"desktop", "/desktop/", http.StatusOK},
		{"installer macos", "/api/installer/macos", http.StatusOK},
		{"installer linux", "/api/installer/linux", http.StatusOK},
		{"installer windows", "/api/installer/windows", http.StatusOK},
		{"recommend", "/api/recommend", http.StatusOK},
		{"toolkit count", "/api/toolkit-count", http.StatusOK},
		{"robots", "/robots.txt", http.StatusOK},
		{"sitemap", "/sitemap.xml", http.StatusOK},
		{"favicon", "/favicon.ico", http.StatusOK},
		{"site asset svg", "/site-assets/assets/screenshots/tool-assay.svg", http.StatusOK},
		{"for solo developers", "/for/solo-developers/", http.StatusOK},
		{"tools listing", "/tools/", http.StatusOK},
		{"playground", "/playground", http.StatusOK},
		{"v1 chat completions", "/v1/chat/completions", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tt.wantCode {
				t.Errorf("path %s: got status %d, want %d", tt.path, w.Code, tt.wantCode)
			}
		})
	}
}
