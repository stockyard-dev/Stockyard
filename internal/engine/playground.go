package engine

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// PlaygroundShare stores a shared playground session.
type PlaygroundShare struct {
	ID        string          `json:"id"`
	Messages  json.RawMessage `json:"messages"`
	Model     string          `json:"model"`
	Provider  string          `json:"provider,omitempty"`
	Modules   json.RawMessage `json:"modules,omitempty"`
	Traces    json.RawMessage `json:"traces,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

const playgroundSchema = `
CREATE TABLE IF NOT EXISTS playground_shares (
	id         TEXT PRIMARY KEY,
	messages   TEXT NOT NULL,
	model      TEXT NOT NULL DEFAULT '',
	provider   TEXT NOT NULL DEFAULT '',
	modules    TEXT NOT NULL DEFAULT '{}',
	traces     TEXT NOT NULL DEFAULT '[]',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	expires_at TEXT NOT NULL DEFAULT (datetime('now', '+30 days'))
);
CREATE INDEX IF NOT EXISTS idx_playground_expires ON playground_shares(expires_at);
`

func migratePlayground(conn *sql.DB) {
	if _, err := conn.Exec(playgroundSchema); err != nil {
		log.Printf("[playground] schema migration: %v", err)
	}
	// Add traces column if upgrading from older schema
	conn.Exec(`ALTER TABLE playground_shares ADD COLUMN traces TEXT NOT NULL DEFAULT '[]'`)
}

func genShareID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// registerPlaygroundRoutes mounts POST /api/playground/share and GET /api/playground/share/{id}.
func registerPlaygroundRoutes(mux *http.ServeMux, conn *sql.DB) {
	migratePlayground(conn)

	// POST /api/playground/share — create a shared session
	mux.HandleFunc("POST /api/playground/share", func(w http.ResponseWriter, r *http.Request) {
		// Limit body to 1MB since this is a public endpoint
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req struct {
			Messages json.RawMessage `json:"messages"`
			Model    string          `json:"model"`
			Provider string          `json:"provider"`
			Modules  json.RawMessage `json:"modules"`
			Traces   json.RawMessage `json:"traces"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if len(req.Messages) == 0 || string(req.Messages) == "null" {
			http.Error(w, `{"error":"messages required"}`, http.StatusBadRequest)
			return
		}

		id := genShareID()
		modules := req.Modules
		if len(modules) == 0 {
			modules = json.RawMessage(`{}`)
		}
		traces := req.Traces
		if len(traces) == 0 {
			traces = json.RawMessage(`[]`)
		}

		_, err := conn.Exec(
			`INSERT INTO playground_shares (id, messages, model, provider, modules, traces) VALUES (?, ?, ?, ?, ?, ?)`,
			id, string(req.Messages), req.Model, req.Provider, string(modules), string(traces),
		)
		if err != nil {
			log.Printf("[playground] insert: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":  id,
			"url": "/playground?share=" + id,
		})
	})

	// GET /api/playground/share/{id} — retrieve a shared session
	mux.HandleFunc("GET /api/playground/share/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}

		var share PlaygroundShare
		var messages, modules, traces, createdAt, expiresAt string
		err := conn.QueryRow(
			`SELECT id, messages, model, provider, modules, traces, created_at, expires_at FROM playground_shares WHERE id = ? AND expires_at > datetime('now')`,
			id,
		).Scan(&share.ID, &messages, &share.Model, &share.Provider, &modules, &traces, &createdAt, &expiresAt)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"not found or expired"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("[playground] query: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		share.Messages = json.RawMessage(messages)
		share.Modules = json.RawMessage(modules)
		share.Traces = json.RawMessage(traces)
		share.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		share.ExpiresAt, _ = time.Parse("2006-01-02 15:04:05", expiresAt)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(share)
	})

	// --- Playground Sessions (persistent history) ---
	conn.Exec(`CREATE TABLE IF NOT EXISTS playground_sessions (
		id TEXT PRIMARY KEY,
		config TEXT NOT NULL DEFAULT '{}',
		messages TEXT NOT NULL DEFAULT '[]',
		results TEXT NOT NULL DEFAULT '[]',
		models TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_pg_sessions_created ON playground_sessions(created_at)`)

	// POST /api/playground/sessions — save a playground session
	mux.HandleFunc("POST /api/playground/sessions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Config   json.RawMessage `json:"config"`
			Messages json.RawMessage `json:"messages"`
			Results  json.RawMessage `json:"results"`
			Models   string          `json:"models"` // comma-separated model names
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		id := genShareID()
		cfgStr := "{}"
		if len(req.Config) > 0 {
			cfgStr = string(req.Config)
		}
		msgsStr := "[]"
		if len(req.Messages) > 0 {
			msgsStr = string(req.Messages)
		}
		resultsStr := "[]"
		if len(req.Results) > 0 {
			resultsStr = string(req.Results)
		}

		conn.Exec(`INSERT INTO playground_sessions (id, config, messages, results, models) VALUES (?, ?, ?, ?, ?)`,
			id, cfgStr, msgsStr, resultsStr, req.Models)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	})

	// GET /api/playground/sessions — list recent sessions
	mux.HandleFunc("GET /api/playground/sessions", func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`SELECT id, config, messages, results, models, created_at
			FROM playground_sessions ORDER BY created_at DESC LIMIT 50`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]any{})
			return
		}
		defer rows.Close()

		var sessions []map[string]any
		for rows.Next() {
			var id, cfgStr, msgsStr, resultsStr, models, createdAt string
			if err := rows.Scan(&id, &cfgStr, &msgsStr, &resultsStr, &models, &createdAt); err != nil {
				continue
			}
			var cfg, msgs, results any
			if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
				log.Printf("[playground] json parse error: %v", err)
			}
			if err := json.Unmarshal([]byte(msgsStr), &msgs); err != nil {
				log.Printf("[playground] json parse error: %v", err)
			}
			if err := json.Unmarshal([]byte(resultsStr), &results); err != nil {
				log.Printf("[playground] json parse error: %v", err)
			}
			sessions = append(sessions, map[string]any{
				"id": id, "config": cfg, "messages": msgs, "results": results,
				"models": models, "created_at": createdAt,
			})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		if sessions == nil {
			sessions = []map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	})

	// GET /api/playground/sessions/{id} — get a specific session
	mux.HandleFunc("GET /api/playground/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var cfgStr, msgsStr, resultsStr, models, createdAt string
		err := conn.QueryRow(`SELECT config, messages, results, models, created_at
			FROM playground_sessions WHERE id = ?`, id).
			Scan(&cfgStr, &msgsStr, &resultsStr, &models, &createdAt)
		if err != nil {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}
		var cfg, msgs, results any
		if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
			log.Printf("[playground] json parse error: %v", err)
		}
		if err := json.Unmarshal([]byte(msgsStr), &msgs); err != nil {
			log.Printf("[playground] json parse error: %v", err)
		}
		if err := json.Unmarshal([]byte(resultsStr), &results); err != nil {
			log.Printf("[playground] json parse error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": id, "config": cfg, "messages": msgs, "results": results,
			"models": models, "created_at": createdAt,
		})
	})

	log.Printf("[playground] session routes registered")

	// Cleanup: remove expired shares periodically
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			result, err := conn.Exec(`DELETE FROM playground_shares WHERE expires_at < datetime('now')`)
			if err == nil {
				if n, _ := result.RowsAffected(); n > 0 {
					log.Printf("[playground] cleaned up %d expired shares", n)
				}
			}
		}
	}()
}
