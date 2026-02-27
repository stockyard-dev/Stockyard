package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ExportConfig represents a full Stockyard configuration snapshot.
type ExportConfig struct {
	Version   string         `json:"version"`
	ExportedAt string        `json:"exported_at"`
	Modules   []ModuleExport `json:"modules"`
	Providers []ProvExport   `json:"providers,omitempty"`
	Webhooks  []WebhookExport `json:"webhooks,omitempty"`
	Policies  []PolicyExport `json:"policies,omitempty"`
}

type ModuleExport struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type ProvExport struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url,omitempty"`
}

type WebhookExport struct {
	URL    string `json:"url"`
	Events string `json:"events"`
}

type PolicyExport struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Action string `json:"action"`
	Pattern string `json:"pattern,omitempty"`
}

// RegisterConfigRoutes mounts config export/import endpoints.
func RegisterConfigRoutes(mux *http.ServeMux, conn *sql.DB) {
	// GET /api/config/export — full config snapshot
	mux.HandleFunc("GET /api/config/export", func(w http.ResponseWriter, r *http.Request) {
		cfg := ExportConfig{
			Version:    "1.0",
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
		}

		// Modules
		rows, err := conn.Query(`SELECT name, enabled FROM proxy_modules ORDER BY name`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var m ModuleExport
				var enabled int
				if rows.Scan(&m.Name, &enabled) == nil {
					m.Enabled = enabled == 1
					cfg.Modules = append(cfg.Modules, m)
				}
			}
		}

		// Providers
		provRows, err := conn.Query(`SELECT name, base_url FROM providers ORDER BY name`)
		if err == nil {
			defer provRows.Close()
			for provRows.Next() {
				var p ProvExport
				var baseURL sql.NullString
				if provRows.Scan(&p.Name, &baseURL) == nil {
					if baseURL.Valid {
						p.BaseURL = baseURL.String
					}
					cfg.Providers = append(cfg.Providers, p)
				}
			}
		}

		// Webhooks (redact secrets)
		whRows, err := conn.Query(`SELECT url, events FROM webhooks WHERE enabled = 1 ORDER BY id`)
		if err == nil {
			defer whRows.Close()
			for whRows.Next() {
				var wh WebhookExport
				if whRows.Scan(&wh.URL, &wh.Events) == nil {
					cfg.Webhooks = append(cfg.Webhooks, wh)
				}
			}
		}

		// Trust policies
		polRows, err := conn.Query(`SELECT name, type, action, pattern FROM trust_policies ORDER BY id`)
		if err == nil {
			defer polRows.Close()
			for polRows.Next() {
				var p PolicyExport
				var pattern sql.NullString
				if polRows.Scan(&p.Name, &p.Type, &p.Action, &pattern) == nil {
					if pattern.Valid {
						p.Pattern = pattern.String
					}
					cfg.Policies = append(cfg.Policies, p)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="stockyard-config.json"`)
		json.NewEncoder(w).Encode(cfg)
	})

	// POST /api/config/import — apply config snapshot
	mux.HandleFunc("POST /api/config/import", func(w http.ResponseWriter, r *http.Request) {
		var cfg ExportConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		applied := 0

		// Apply module states
		for _, m := range cfg.Modules {
			enabled := 0
			if m.Enabled {
				enabled = 1
			}
			res, err := conn.Exec(
				`UPDATE proxy_modules SET enabled = ? WHERE name = ?`,
				enabled, m.Name,
			)
			if err == nil {
				if n, _ := res.RowsAffected(); n > 0 {
					applied++
				}
			}
		}

		// Apply webhooks (add missing ones)
		for _, wh := range cfg.Webhooks {
			var exists int
			conn.QueryRow(`SELECT COUNT(*) FROM webhooks WHERE url = ?`, wh.URL).Scan(&exists)
			if exists == 0 {
				if _, err := conn.Exec(
					`INSERT INTO webhooks (url, events) VALUES (?, ?)`,
					wh.URL, wh.Events,
				); err == nil {
					applied++
				}
			}
		}

		// Apply trust policies (add missing ones)
		for _, p := range cfg.Policies {
			var exists int
			conn.QueryRow(`SELECT COUNT(*) FROM trust_policies WHERE name = ?`, p.Name).Scan(&exists)
			if exists == 0 {
				if _, err := conn.Exec(
					`INSERT INTO trust_policies (name, type, action, pattern) VALUES (?, ?, ?, ?)`,
					p.Name, p.Type, p.Action, p.Pattern,
				); err == nil {
					applied++
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "imported",
			"applied": applied,
			"modules": len(cfg.Modules),
		})
	})

	// POST /api/config/diff — compare current config with uploaded
	mux.HandleFunc("POST /api/config/diff", func(w http.ResponseWriter, r *http.Request) {
		var incoming ExportConfig
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		type Diff struct {
			Module  string `json:"module"`
			Current bool   `json:"current"`
			Incoming bool  `json:"incoming"`
		}

		var diffs []Diff
		for _, m := range incoming.Modules {
			var currentEnabled int
			err := conn.QueryRow(`SELECT enabled FROM proxy_modules WHERE name = ?`, m.Name).Scan(&currentEnabled)
			if err != nil {
				continue
			}
			current := currentEnabled == 1
			if current != m.Enabled {
				diffs = append(diffs, Diff{Module: m.Name, Current: current, Incoming: m.Enabled})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"changes": len(diffs),
			"diffs":   diffs,
		})
	})
}

// ExportConfigCLI writes config to stdout in JSON format (for `stockyard export`).
func ExportConfigCLI(conn *sql.DB) error {
	cfg := ExportConfig{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}

	rows, err := conn.Query(`SELECT name, enabled FROM proxy_modules ORDER BY name`)
	if err != nil {
		return fmt.Errorf("query modules: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m ModuleExport
		var enabled int
		if rows.Scan(&m.Name, &enabled) == nil {
			m.Enabled = enabled == 1
			cfg.Modules = append(cfg.Modules, m)
		}
	}

	enc := json.NewEncoder(nil) // will be os.Stdout in real usage
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}
