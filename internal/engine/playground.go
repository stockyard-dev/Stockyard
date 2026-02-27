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
	ID        string    `json:"id"`
	Messages  json.RawMessage `json:"messages"`
	Model     string    `json:"model"`
	Provider  string    `json:"provider,omitempty"`
	Modules   json.RawMessage `json:"modules,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

const playgroundSchema = `
CREATE TABLE IF NOT EXISTS playground_shares (
	id         TEXT PRIMARY KEY,
	messages   TEXT NOT NULL,
	model      TEXT NOT NULL DEFAULT '',
	provider   TEXT NOT NULL DEFAULT '',
	modules    TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	expires_at TEXT NOT NULL DEFAULT (datetime('now', '+30 days'))
);
CREATE INDEX IF NOT EXISTS idx_playground_expires ON playground_shares(expires_at);
`

func migratePlayground(conn *sql.DB) {
	if _, err := conn.Exec(playgroundSchema); err != nil {
		log.Printf("[playground] schema migration: %v", err)
	}
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
		var req struct {
			Messages json.RawMessage `json:"messages"`
			Model    string          `json:"model"`
			Provider string          `json:"provider"`
			Modules  json.RawMessage `json:"modules"`
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

		_, err := conn.Exec(
			`INSERT INTO playground_shares (id, messages, model, provider, modules) VALUES (?, ?, ?, ?, ?)`,
			id, string(req.Messages), req.Model, req.Provider, string(modules),
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
		var messages, modules, createdAt, expiresAt string
		err := conn.QueryRow(
			`SELECT id, messages, model, provider, modules, created_at, expires_at FROM playground_shares WHERE id = ? AND expires_at > datetime('now')`,
			id,
		).Scan(&share.ID, &messages, &share.Model, &share.Provider, &modules, &createdAt, &expiresAt)
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
		share.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		share.ExpiresAt, _ = time.Parse("2006-01-02 15:04:05", expiresAt)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(share)
	})

	// Cleanup: remove expired shares periodically
	go func() {
		for {
			time.Sleep(6 * time.Hour)
			result, err := conn.Exec(`DELETE FROM playground_shares WHERE expires_at < datetime('now')`)
			if err == nil {
				if n, _ := result.RowsAffected(); n > 0 {
					log.Printf("[playground] cleaned up %d expired shares", n)
				}
			}
		}
	}()
}
