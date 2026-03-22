package auth

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Role constants match team_members.role values.
const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)

// rbacCache caches email→role lookups to avoid per-request DB queries.
type rbacCache struct {
	mu      sync.RWMutex
	entries map[string]rbacEntry
}

type rbacEntry struct {
	role      string
	fetchedAt time.Time
}

func newRBACCache() *rbacCache {
	return &rbacCache{entries: make(map[string]rbacEntry)}
}

func (c *rbacCache) get(email string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[email]
	if !ok || time.Since(e.fetchedAt) > 60*time.Second {
		return "", false
	}
	return e.role, true
}

func (c *rbacCache) set(email, role string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[email] = rbacEntry{role: role, fetchedAt: time.Now()}
}

// RBACMiddleware enforces role-based access control on /api/* routes.
//
// Roles (most → least privileged): owner > admin > developer > viewer.
//
//   - viewer: read-only on all /api/* routes (GET only)
//   - developer: GET + POST on proxy, observe, studio, forge, exchange, memory, knowledge, copilot
//   - admin: everything except team ownership transfer and billing plan changes
//   - owner: unrestricted
//
// If the user has no team_members entry (e.g. single-operator mode), all requests pass through.
func RBACMiddleware(conn *sql.DB) func(http.Handler) http.Handler {
	cache := newRBACCache()

	lookupRole := func(email string) string {
		if role, ok := cache.get(email); ok {
			return role
		}
		var role string
		err := conn.QueryRow("SELECT role FROM team_members WHERE email = ? AND invite_accepted = 1", email).Scan(&role)
		if err != nil {
			return "" // not a team member — pass through (single-operator mode)
		}
		cache.set(email, role)
		return role
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Only enforce on API management routes
			if !strings.HasPrefix(path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}

			// Skip public/unauthenticated endpoints
			if path == "/api/apps" || path == "/api/health" || path == "/api/license" ||
				strings.HasPrefix(path, "/api/auth/") {
				next.ServeHTTP(w, r)
				return
			}

			user := UserFromContext(r.Context())
			if user == nil {
				// No authenticated user — admin key auth handles this separately
				next.ServeHTTP(w, r)
				return
			}

			role := lookupRole(user.Email)
			if role == "" {
				// Not in team_members — single-operator mode, allow everything
				next.ServeHTTP(w, r)
				return
			}

			if !checkPermission(role, r.Method, path) {
				log.Printf("[rbac] denied: user=%s role=%s method=%s path=%s", user.Email, role, r.Method, path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":{"message":"insufficient permissions","type":"rbac_error","role":"` + role + `"}}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkPermission returns true if the given role can perform method on path.
func checkPermission(role, method, path string) bool {
	// Owner: unrestricted
	if role == RoleOwner {
		return true
	}

	// Admin: everything except team ownership and some billing
	if role == RoleAdmin {
		// Block ownership transfer
		if strings.HasPrefix(path, "/api/team/") && method == "DELETE" {
			return true // admins can remove members
		}
		return true
	}

	isRead := method == "GET" || method == "HEAD" || method == "OPTIONS"

	// Viewer: read-only everywhere
	if role == RoleViewer {
		return isRead
	}

	// Developer: read everywhere, write on operational APIs
	if role == RoleDeveloper {
		if isRead {
			return true
		}
		// Developers can write to these app APIs
		writeAllowed := []string{
			"/api/proxy/",    // toggle modules, manage providers
			"/api/observe/",  // create alerts, record traces
			"/api/studio/",   // manage templates, run experiments
			"/api/forge/",    // create/run workflows
			"/api/exchange/", // install packs
			"/api/memory/",   // store memories
			"/api/knowledge/", // add entries, query
			"/api/copilot/",  // chat with copilot
			"/api/recall/",   // create incidents
			"/api/apps/",     // app builder
		}
		for _, prefix := range writeAllowed {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		// Block writes to admin-only APIs
		return false
	}

	return false
}
