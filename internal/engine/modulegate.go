// Package engine — modulegate provides a concurrent-safe runtime toggle
// for proxy middleware modules. Each middleware is wrapped with a gate
// check; when a module is disabled via the API, requests skip it.
package engine

import (
	"database/sql"
	"log"
	"sync"

	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ModuleGate tracks which middleware modules are enabled at runtime.
// It is safe for concurrent reads from request goroutines and writes
// from the management API.
type ModuleGate struct {
	mu      sync.RWMutex
	enabled map[string]bool
}

// NewModuleGate creates a gate with all modules enabled by default.
func NewModuleGate() *ModuleGate {
	return &ModuleGate{enabled: make(map[string]bool)}
}

// LoadFromDB reads the proxy_modules table and populates the gate.
// Called once at boot after migrations have run.
func (g *ModuleGate) LoadFromDB(conn *sql.DB) {
	rows, err := conn.Query("SELECT name, enabled FROM proxy_modules")
	if err != nil {
		return
	}
	defer rows.Close()

	g.mu.Lock()
	defer g.mu.Unlock()
	for rows.Next() {
		var name string
		var enabled int
		rows.Scan(&name, &enabled)
		g.enabled[name] = enabled == 1
	}
	log.Printf("[modulegate] loaded %d modules from DB", len(g.enabled))
}

// Set updates a module's enabled state. Called by the Proxy app's
// PUT /api/proxy/modules/{name} handler after writing to SQLite.
func (g *ModuleGate) Set(name string, enabled bool) {
	g.mu.Lock()
	g.enabled[name] = enabled
	g.mu.Unlock()
	log.Printf("[modulegate] %s → %v", name, enabled)
}

// IsEnabled returns whether a module should execute. If the module
// is unknown (not in the gate), it defaults to enabled so that
// middleware not in the DB still runs.
func (g *ModuleGate) IsEnabled(name string) bool {
	g.mu.RLock()
	enabled, ok := g.enabled[name]
	g.mu.RUnlock()
	if !ok {
		return true // unknown modules default to enabled
	}
	return enabled
}

// Gated wraps a middleware so it only executes when the named module
// is enabled in the gate. When disabled, requests pass straight through
// to the next middleware with zero overhead beyond the map lookup.
func (g *ModuleGate) Gated(name string, mw proxy.Middleware) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		// Build the "active" handler once (the wrapped chain with this middleware)
		active := mw(next)
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if g.IsEnabled(name) {
				return active(ctx, req)
			}
			return next(ctx, req)
		}
	}
}
