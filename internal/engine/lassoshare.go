// Package engine — lassoshare creates shareable URLs from Lasso replay comparisons.
// Each share snapshots the comparison data so the URL works forever (30 days),
// even if the underlying replay run is deleted.
// The /compare/{id} page renders the share as a branded, embeddable card.
package engine

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const lassoShareSchema = `
CREATE TABLE IF NOT EXISTS lasso_shares (
	id          TEXT PRIMARY KEY,
	replay_id   TEXT NOT NULL DEFAULT '',
	prompt      TEXT NOT NULL DEFAULT '',
	left_model  TEXT NOT NULL,
	left_prov   TEXT NOT NULL DEFAULT '',
	left_resp   TEXT NOT NULL DEFAULT '',
	left_tokens INTEGER DEFAULT 0,
	left_cost   REAL DEFAULT 0,
	left_ms     INTEGER DEFAULT 0,
	right_model TEXT NOT NULL,
	right_prov  TEXT NOT NULL DEFAULT '',
	right_resp  TEXT NOT NULL DEFAULT '',
	right_tokens INTEGER DEFAULT 0,
	right_cost  REAL DEFAULT 0,
	right_ms    INTEGER DEFAULT 0,
	winners     TEXT NOT NULL DEFAULT '{}',
	title       TEXT NOT NULL DEFAULT '',
	created_at  TEXT NOT NULL DEFAULT (datetime('now')),
	expires_at  TEXT NOT NULL DEFAULT (datetime('now', '+30 days')),
	views       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_lasso_shares_expires ON lasso_shares(expires_at);
`

func genLassoShareID() string {
	b := make([]byte, 6) // 12 hex chars — short enough for URLs
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RegisterLassoShareRoutes mounts share creation and retrieval endpoints.
func RegisterLassoShareRoutes(mux *http.ServeMux, conn *sql.DB) {
	if _, err := conn.Exec(lassoShareSchema); err != nil {
		log.Printf("[schema] migration error: %v", err)
	}

	// POST /api/replay/share — create a share from a replay run ID
	mux.HandleFunc("POST /api/replay/share", handleCreateLassoShare(conn))
	mux.HandleFunc("POST /api/lasso/share", handleCreateLassoShare(conn))

	// POST /api/lasso/share/custom — create a share from raw comparison data (no replay run needed)
	mux.HandleFunc("POST /api/lasso/share/custom", handleCreateCustomShare(conn))

	// GET /api/lasso/share/{id} — retrieve share data (JSON, for the compare page to fetch)
	mux.HandleFunc("GET /api/lasso/share/{id}", handleGetLassoShare(conn))
	mux.HandleFunc("GET /api/replay/share/{id}", handleGetLassoShare(conn))

	// Cleanup expired shares
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			result, err := conn.Exec(`DELETE FROM lasso_shares WHERE expires_at < datetime('now')`)
			if err == nil {
				if n, _ := result.RowsAffected(); n > 0 {
					log.Printf("[lasso-share] cleaned up %d expired shares", n)
				}
			}
		}
	}()

	log.Printf("[lasso-share] share routes registered")
}

// handleCreateLassoShare snapshots a replay run into a shareable URL.
func handleCreateLassoShare(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ReplayID string `json:"replay_id"`
			Title    string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			shareJSON(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.ReplayID == "" {
			shareJSON(w, 400, map[string]string{"error": "replay_id required"})
			return
		}

		// Fetch the replay run
		var srcModel, srcProv, tgtModel, tgtProv string
		var srcResp, tgtResp, reqBody, status string
		var srcTokIn, srcTokOut, srcDurMs, tgtTokIn, tgtTokOut, tgtDurMs int64
		var srcCost, tgtCost float64

		err := conn.QueryRow(`SELECT source_model, source_provider,
			target_model, target_provider, source_response, target_response,
			request_body,
			source_tokens_in, source_tokens_out, source_cost_usd, source_duration_ms,
			target_tokens_in, target_tokens_out, target_cost_usd, target_duration_ms, status
			FROM replay_runs WHERE id = ?`, req.ReplayID).Scan(
			&srcModel, &srcProv, &tgtModel, &tgtProv, &srcResp, &tgtResp,
			&reqBody,
			&srcTokIn, &srcTokOut, &srcCost, &srcDurMs,
			&tgtTokIn, &tgtTokOut, &tgtCost, &tgtDurMs, &status)
		if err != nil {
			shareJSON(w, 404, map[string]string{"error": "replay run not found"})
			return
		}
		if status != "ok" {
			shareJSON(w, 422, map[string]string{"error": "can only share successful replays"})
			return
		}

		// Extract prompt from request body
		prompt := extractPromptFromBody(reqBody)

		// Determine winners
		costWinner := srcModel
		if tgtCost < srcCost {
			costWinner = tgtModel
		}
		speedWinner := srcModel
		if tgtDurMs < srcDurMs {
			speedWinner = tgtModel
		}

		winnersJSON, _ := json.Marshal(map[string]string{
			"cost":  costWinner,
			"speed": speedWinner,
		})

		// Generate title if not provided
		title := req.Title
		if title == "" {
			title = fmt.Sprintf("%s vs %s", srcModel, tgtModel)
		}

		id := genLassoShareID()
		_, err = conn.Exec(`INSERT INTO lasso_shares
			(id, replay_id, prompt, left_model, left_prov, left_resp, left_tokens, left_cost, left_ms,
			 right_model, right_prov, right_resp, right_tokens, right_cost, right_ms, winners, title)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, req.ReplayID, prompt,
			srcModel, srcProv, truncateShare(srcResp, 2000), srcTokIn+srcTokOut, srcCost, srcDurMs,
			tgtModel, tgtProv, truncateShare(tgtResp, 2000), tgtTokIn+tgtTokOut, tgtCost, tgtDurMs,
			string(winnersJSON), title)
		if err != nil {
			log.Printf("[lasso-share] insert error: %v", err)
			shareJSON(w, 500, map[string]string{"error": "failed to create share"})
			return
		}

		shareURL := "/compare/" + id
		fullURL := "https://stockyard.dev/compare/" + id

		shareJSON(w, 201, map[string]any{
			"id":        id,
			"url":       shareURL,
			"full_url":  fullURL,
			"title":     title,
			"expires":   time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		})
	}
}

// handleCreateCustomShare creates a share from raw data (no replay run needed).
// This lets the playground/dashboard create shares from any comparison.
func handleCreateCustomShare(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Title      string `json:"title"`
			Prompt     string `json:"prompt"`
			LeftModel  string `json:"left_model"`
			LeftProv   string `json:"left_provider"`
			LeftResp   string `json:"left_response"`
			LeftTokens int64  `json:"left_tokens"`
			LeftCost   float64 `json:"left_cost"`
			LeftMs     int64  `json:"left_ms"`
			RightModel string `json:"right_model"`
			RightProv  string `json:"right_provider"`
			RightResp  string `json:"right_response"`
			RightTokens int64 `json:"right_tokens"`
			RightCost  float64 `json:"right_cost"`
			RightMs    int64  `json:"right_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			shareJSON(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.LeftModel == "" || req.RightModel == "" {
			shareJSON(w, 400, map[string]string{"error": "left_model and right_model required"})
			return
		}

		costWinner := req.LeftModel
		if req.RightCost < req.LeftCost {
			costWinner = req.RightModel
		}
		speedWinner := req.LeftModel
		if req.RightMs < req.LeftMs {
			speedWinner = req.RightModel
		}
		winnersJSON, marshalErr := json.Marshal(map[string]string{"cost": costWinner, "speed": speedWinner})
		if marshalErr != nil {
			winnersJSON = []byte("{}")
		}

		title := req.Title
		if title == "" {
			title = fmt.Sprintf("%s vs %s", req.LeftModel, req.RightModel)
		}

		id := genLassoShareID()
		conn.Exec(`INSERT INTO lasso_shares
			(id, prompt, left_model, left_prov, left_resp, left_tokens, left_cost, left_ms,
			 right_model, right_prov, right_resp, right_tokens, right_cost, right_ms, winners, title)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, req.Prompt,
			req.LeftModel, req.LeftProv, truncateShare(req.LeftResp, 2000), req.LeftTokens, req.LeftCost, req.LeftMs,
			req.RightModel, req.RightProv, truncateShare(req.RightResp, 2000), req.RightTokens, req.RightCost, req.RightMs,
			string(winnersJSON), title)

		shareURL := "/compare/" + id
		fullURL := "https://stockyard.dev/compare/" + id

		shareJSON(w, 201, map[string]any{
			"id":       id,
			"url":      shareURL,
			"full_url": fullURL,
			"title":    title,
			"expires":  time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		})
	}
}

// handleGetLassoShare retrieves share data and increments view count.
func handleGetLassoShare(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			shareJSON(w, 400, map[string]string{"error": "id required"})
			return
		}

		var prompt, leftModel, leftProv, leftResp, rightModel, rightProv, rightResp string
		var winnersStr, title, createdAt string
		var leftTokens, rightTokens, leftMs, rightMs, views int64
		var leftCost, rightCost float64

		err := conn.QueryRow(`SELECT prompt, left_model, left_prov, left_resp, left_tokens, left_cost, left_ms,
			right_model, right_prov, right_resp, right_tokens, right_cost, right_ms,
			winners, title, created_at, views
			FROM lasso_shares WHERE id = ? AND expires_at > datetime('now')`, id).Scan(
			&prompt, &leftModel, &leftProv, &leftResp, &leftTokens, &leftCost, &leftMs,
			&rightModel, &rightProv, &rightResp, &rightTokens, &rightCost, &rightMs,
			&winnersStr, &title, &createdAt, &views)
		if err != nil {
			shareJSON(w, 404, map[string]string{"error": "comparison not found or expired"})
			return
		}

		// Increment view count
		conn.Exec(`UPDATE lasso_shares SET views = views + 1 WHERE id = ?`, id)

		var winners map[string]string
		if err := json.Unmarshal([]byte(winnersStr), &winners); err != nil {
			log.Printf("[lassoshare] json parse error: %v", err)
		}

		// Cost savings calculation
		costDelta := rightCost - leftCost
		costPct := pctChange(leftCost, rightCost)

		shareJSON(w, 200, map[string]any{
			"id":    id,
			"title": title,
			"prompt": prompt,
			"left": map[string]any{
				"model":      leftModel,
				"provider":   leftProv,
				"response":   leftResp,
				"tokens":     leftTokens,
				"cost_usd":   leftCost,
				"latency_ms": leftMs,
			},
			"right": map[string]any{
				"model":      rightModel,
				"provider":   rightProv,
				"response":   rightResp,
				"tokens":     rightTokens,
				"cost_usd":   rightCost,
				"latency_ms": rightMs,
			},
			"winners": winners,
			"delta": map[string]any{
				"cost_usd": costDelta,
				"cost_pct": costPct,
			},
			"views":      views + 1,
			"created_at": createdAt,
			"share_url":  "https://stockyard.dev/compare/" + id,
		})
	}
}

func extractPromptFromBody(body string) string {
	var parsed struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		log.Printf("[lassoshare] json parse error: %v", err)
	}
	for _, m := range parsed.Messages {
		if m.Role == "user" {
			if len(m.Content) > 500 {
				return m.Content[:500] + "..."
			}
			return m.Content
		}
	}
	return ""
}

func truncateShare(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// shareRedactResponse trims response for OG previews
func sharePreview(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

func shareJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
