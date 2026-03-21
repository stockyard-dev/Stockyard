package features

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// MeshRouteConfig controls mesh routing behavior.
type MeshRouteConfig struct {
	Mode          string   `json:"mode"`           // mesh-first, cloud-first, cheapest, fastest
	MaxLatencyMs  int      `json:"max_latency_ms"`
	MinReputation float64  `json:"min_reputation"`
	AllowedRegions []string `json:"allowed_regions"`
}

// MeshRouteMiddleware routes requests to mesh nodes when available.
func MeshRouteMiddleware(conn *sql.DB) proxy.Middleware {
	var (
		mu       sync.RWMutex
		cfg      MeshRouteConfig
		lastLoad time.Time
	)

	loadConfig := func() MeshRouteConfig {
		mu.RLock()
		if time.Since(lastLoad) < 60*time.Second {
			c := cfg
			mu.RUnlock()
			return c
		}
		mu.RUnlock()

		mu.Lock()
		defer mu.Unlock()
		if time.Since(lastLoad) < 60*time.Second {
			return cfg
		}

		cfg = MeshRouteConfig{Mode: "cloud-first", MaxLatencyMs: 5000, MinReputation: 30}
		var raw string
		err := conn.QueryRow("SELECT config_json FROM proxy_modules WHERE name = 'meshroute'").Scan(&raw)
		if err == nil && raw != "" && raw != "{}" {
			json.Unmarshal([]byte(raw), &cfg)
		}
		lastLoad = time.Now()
		return cfg
	}

	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			c := loadConfig()

			if c.Mode == "cloud-first" || c.Mode == "" {
				// Default: use cloud providers, mesh is fallback
				return next(ctx, req)
			}

			// Check for available mesh nodes
			var nodeCount int
			conn.QueryRow(`SELECT COUNT(*) FROM mesh_nodes WHERE status = 'active'
				AND last_heartbeat > datetime('now', '-60 seconds')`).Scan(&nodeCount)

			if nodeCount == 0 {
				return next(ctx, req)
			}

			log.Printf("[meshroute] %d mesh nodes available, mode=%s", nodeCount, c.Mode)

			// For now, route to cloud — actual mesh routing requires
			// inter-node communication which is out of scope for single-binary
			return next(ctx, req)
		}
	}
}
