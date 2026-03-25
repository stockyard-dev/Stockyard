package platform

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/license"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/hub"
)

// MountConfig holds all shared resources that products receive.
type MountConfig struct {
	Mux      *http.ServeMux
	Tier     Tier
	DB       *sql.DB
	EventBus *bus.Bus
	Hub      *hub.Hub
	Registry *ProductRegistry
}

// ProductServer wraps a product's http.Handler with its ID.
// Products must implement http.Handler (via ServeHTTP on their Server type).
type ProductServer struct {
	ID      string
	Handler http.Handler
}

// Mount registers all product HTTP handlers on the main mux.
// Products below the active tier get the Gate middleware (returns 403).
// Products at or above the tier get mounted normally.
// All products are visible in the registry regardless of tier.
func Mount(cfg MountConfig, servers []ProductServer) {
	tier := cfg.Tier

	// Register all products in the hub
	if cfg.Hub != nil {
		hub.RegisterAll(cfg.Hub)
	}

	// Register all products in the platform registry
	cfg.Registry.RegisterAll(tier)

	// Mount each product server
	for _, ps := range servers {
		mountProduct(cfg.Mux, ps.ID, tier, ps.Handler, cfg.Registry)
	}

	// Mount platform-level API endpoints (available at all tiers)
	cfg.Mux.HandleFunc("GET /api/platform/health", HandleHealth(cfg.Registry, tier))
	cfg.Mux.HandleFunc("GET /api/platform/products", cfg.Registry.HandleProducts(tier))
	cfg.Mux.HandleFunc("PUT /api/platform/products/{id}/toggle", cfg.Registry.HandleToggle())

	// Event stream (SSE) from the bus
	if cfg.EventBus != nil {
		cfg.Mux.HandleFunc("GET /api/platform/events", handleEventStream(cfg.EventBus))
	}

	// Workflow listing from orchestrator
	if cfg.Hub != nil {
		cfg.Mux.HandleFunc("GET /api/platform/workflows", func(w http.ResponseWriter, r *http.Request) {
			graph := cfg.Hub.DependencyGraph()
			summary := cfg.Hub.Summary()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"products":    summary,
				"edges":       graph,
				"edge_count":  len(graph),
			})
		})
	}

	// Bus stats
	if cfg.EventBus != nil {
		cfg.Mux.HandleFunc("GET /api/platform/bus", func(w http.ResponseWriter, r *http.Request) {
			stats := cfg.EventBus.Stats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
		})
	}

	// Summary log
	active := len(cfg.Registry.Active())
	total := cfg.Registry.Count()
	log.Printf("[platform] mounted %d/%d products (tier=%s)", active, total, TierName(tier))
}

// mountProduct puts a product's handler on the mux at /{id}/.
// If the tier is insufficient, a gate handler is mounted instead.
func mountProduct(mux *http.ServeMux, id string, tier Tier, handler http.Handler, reg *ProductRegistry) {
	requiredTier := ProductTiers[id]
	prefix := "/" + id + "/"

	if tier >= requiredTier {
		// Active: mount the real product with prefix stripping
		mux.Handle(prefix, http.StripPrefix("/"+id, handler))
		log.Printf("[platform]   ✓ %s (tier=%s)", id, TierName(requiredTier))
	} else {
		// Locked: mount the gate
		mux.Handle(prefix, Gate(tier, id))
		log.Printf("[platform]   ✗ %s (requires %s, have %s)", id, TierName(requiredTier), TierName(tier))
	}
}

// ResolveTier determines the active tier from environment and license.
func ResolveTier(lic *license.License) Tier {
	// Priority 1: STOCKYARD_LICENSE env var (dev mode)
	if envLic := os.Getenv("STOCKYARD_LICENSE"); envLic != "" {
		switch strings.ToLower(envLic) {
		case "dev", "development":
			return TierDev
		case "enterprise":
			return TierEnterprise
		case "team":
			return TierTeam
		case "pro":
			return TierPro
		case "individual":
			return TierIndividual
		case "community", "free":
			return TierCommunity
		}
	}

	// Priority 2: License key
	if lic != nil && lic.Valid {
		return TierFromLicense(lic.Payload.Tier)
	}

	// Default: Community
	return TierCommunity
}

// MigrateSchema creates the platform-level SQLite tables.
func MigrateSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS platform_license (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			tier TEXT NOT NULL DEFAULT 'community',
			license_key TEXT,
			customer_id TEXT,
			expires_at DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS platform_product_state (
			product_id TEXT PRIMARY KEY,
			enabled INTEGER DEFAULT 1,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS platform_events (
			id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			source TEXT NOT NULL,
			data TEXT,
			trace_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_events_type ON platform_events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_events_source ON platform_events(source)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// handleEventStream returns an SSE handler that streams bus events.
func handleEventStream(eventBus *bus.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// Send recent history first
		history := eventBus.History(50, "", "")
		for _, evt := range history {
			data, _ := json.Marshal(evt)
			w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		flusher.Flush()

		// Subscribe to live events
		ch := make(chan bus.Event, 64)
		subID := eventBus.Subscribe("", "", func(evt bus.Event) {
			select {
			case ch <- evt:
			default:
				// Drop if buffer full
			}
		})
		defer eventBus.Unsubscribe(subID)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-ch:
				data, _ := json.Marshal(evt)
				w.Write([]byte("data: " + string(data) + "\n\n"))
				flusher.Flush()
			}
		}
	}
}
