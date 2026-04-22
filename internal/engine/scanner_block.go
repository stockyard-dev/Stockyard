package engine

import (
	"net/http"
	"strings"
)

// scannerBlockPatterns are URL path substrings that only credential/exploit
// scanners ever request. The marketing site has no real route that matches
// any of these, so blocking is safe and stops bots from probing for leaked
// .env files, phpinfo dumps, WordPress installs, etc.
//
// Observed in the wild against stockyard.dev (Cloudflare analytics, 2026-04):
// ~60 unique scanner paths/day driving ~75% of all 404s. None hit real routes.
var scannerBlockPatterns = []string{
	".env",        // catches /.env, /.env.bak, /.env.production, /backend/.env, etc.
	"phpinfo",     // catches /phpinfo.php, /old_phpinfo.php, /hosting/phpinfo.php, etc.
	"wp-admin",    // catches /wp-admin/install.php and any wp-admin probe
	"wp-login",    // catches /wp-login.php
	"/debug.php",  // exact-match scanner path
	"/php.php",    // exact-match scanner path
}

// scannerBlockMiddleware short-circuits requests for known credential/exploit
// scanner paths with a 403, before any handler, template, or DB work runs.
//
// Wrap this OUTERMOST (last WrapHandler call) so it runs first on every
// request. Cost per blocked request: a handful of strings.Contains calls.
func scannerBlockMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		for _, pat := range scannerBlockPatterns {
			if strings.Contains(path, pat) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
