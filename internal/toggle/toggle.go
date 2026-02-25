// Package toggle provides a runtime middleware toggle registry.
// Each middleware is wrapped with a check. When disabled via the API,
// requests pass straight through without executing that middleware.
package toggle

import (
	"context"
	"database/sql"
	"log"
	"sync"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Registry holds runtime enabled/disabled state for named middleware.
// Thread-safe for concurrent reads (hot path) and occasional writes (API toggle).
type Registry struct {
	mu    sync.RWMutex
	state map[string]bool
}

// New creates an empty registry.
func New() *Registry {
	return &Registry{state: make(map[string]bool)}
}

// Set updates a module's enabled state.
func (r *Registry) Set(name string, enabled bool) {
	r.mu.Lock()
	r.state[name] = enabled
	r.mu.Unlock()
	log.Printf("[toggle] %s → %v", name, enabled)
}

// Enabled returns whether a module is enabled. Unknown = true (safe default).
func (r *Registry) Enabled(name string) bool {
	r.mu.RLock()
	v, ok := r.state[name]
	r.mu.RUnlock()
	if !ok {
		return true
	}
	return v
}

// SeedFromDB loads initial state from proxy_modules table.
func (r *Registry) SeedFromDB(conn *sql.DB) {
	rows, err := conn.Query("SELECT name, enabled FROM proxy_modules")
	if err != nil {
		return
	}
	defer rows.Close()
	r.mu.Lock()
	for rows.Next() {
		var name string
		var enabled int
		if err := rows.Scan(&name, &enabled); err == nil {
			r.state[name] = enabled == 1
		}
	}
	r.mu.Unlock()
	log.Printf("[toggle] seeded %d module states", len(r.state))
}

// Wrap returns a middleware that skips execution when the named module is disabled.
func Wrap(name string, reg *Registry, mw proxy.Middleware) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		inner := mw(next)
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if !reg.Enabled(name) {
				return next(ctx, req)
			}
			return inner(ctx, req)
		}
	}
}

// Global is the package-level toggle registry, set by the engine at boot.
// The proxy app reads this to update live middleware state on PUT.
var Global *Registry
