package platform

import (
	"database/sql"
	"log"
	"sync"
)

// ModuleGate tracks which middleware modules are enabled at runtime.
// Goroutine-safe. The engine wraps each middleware with a gate check;
// the proxy app's PUT handler calls SetEnabled to toggle at runtime.
type ModuleGate struct {
	mu      sync.RWMutex
	enabled map[string]bool
}

// NewModuleGate creates a gate with an empty state map.
func NewModuleGate() *ModuleGate {
	return &ModuleGate{enabled: make(map[string]bool)}
}

// LoadFromDB reads the proxy_modules table and populates the in-memory map.
func (g *ModuleGate) LoadFromDB(conn *sql.DB) {
	rows, err := conn.Query("SELECT name, enabled FROM proxy_modules")
	if err != nil {
		log.Printf("[modulegate] load: %v", err)
		return
	}
	defer rows.Close()

	g.mu.Lock()
	defer g.mu.Unlock()
	count := 0
	for rows.Next() {
		var name string
		var enabled int
		if err := rows.Scan(&name, &enabled); err != nil {
			continue
		}
		g.enabled[name] = enabled == 1
		count++
	}
	log.Printf("[modulegate] loaded %d module states from DB", count)
}

// IsEnabled returns whether a module is enabled. Unknown modules default to enabled.
func (g *ModuleGate) IsEnabled(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if v, ok := g.enabled[name]; ok {
		return v
	}
	return true
}

// SetEnabled updates the in-memory state for a module.
// The caller is responsible for also persisting to the DB.
func (g *ModuleGate) SetEnabled(name string, enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.enabled[name] = enabled
	log.Printf("[modulegate] %s → enabled=%v (live)", name, enabled)
}

// Gate is the global module gate instance, initialized by the engine at boot.
var Gate *ModuleGate
