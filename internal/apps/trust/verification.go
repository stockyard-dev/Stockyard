package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const verificationSchema = `
CREATE TABLE IF NOT EXISTS trust_anchors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chain_hash TEXT NOT NULL,
    event_count INTEGER NOT NULL,
    published_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) migrateVerification() {
	a.conn.Exec(verificationSchema)
}

func (a *App) registerVerificationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/trust/publish", a.handlePublishAnchor)
	mux.HandleFunc("GET /api/trust/verify-external", a.handleVerifyExternal)
	mux.HandleFunc("GET /api/trust/anchors", a.handleListAnchors)
	log.Printf("[trust] verification routes registered")
}

func (a *App) handlePublishAnchor(w http.ResponseWriter, r *http.Request) {
	// Compute hash of entire chain state.
	rows, err := a.conn.Query("SELECT hash FROM trust_ledger ORDER BY id")
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read chain"})
		return
	}
	defer rows.Close()

	h := sha256.New()
	count := 0
	for rows.Next() {
		var hash string
		rows.Scan(&hash)
		h.Write([]byte(hash))
		count++
	}
	chainHash := hex.EncodeToString(h.Sum(nil))

	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("INSERT INTO trust_anchors (chain_hash, event_count, published_at) VALUES (?, ?, ?)",
		chainHash, count, now)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"chain_hash":  chainHash,
		"event_count": count,
		"published_at": now,
	})
}

func (a *App) handleVerifyExternal(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "hash query parameter required"})
		return
	}

	// Check if this hash exists in our anchors.
	var publishedAt string
	var eventCount int
	err := a.conn.QueryRow("SELECT event_count, published_at FROM trust_anchors WHERE chain_hash = ?", hash).
		Scan(&eventCount, &publishedAt)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"verified":   false,
			"hash":       hash,
			"message":    "Hash not found in published anchors",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"verified":     true,
		"hash":         hash,
		"event_count":  eventCount,
		"published_at": publishedAt,
	})
}

func (a *App) handleListAnchors(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT id, chain_hash, event_count, published_at FROM trust_anchors ORDER BY published_at DESC LIMIT 50")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"anchors": []any{}})
		return
	}
	defer rows.Close()

	var anchors []map[string]any
	for rows.Next() {
		var id, hash, publishedAt string
		var eventCount int
		rows.Scan(&id, &hash, &eventCount, &publishedAt)
		anchors = append(anchors, map[string]any{
			"id": id, "chain_hash": hash, "event_count": eventCount, "published_at": publishedAt,
		})
	}
	if anchors == nil {
		anchors = []map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"anchors": anchors})
}
