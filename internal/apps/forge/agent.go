package forge

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const agentSchema = `
CREATE TABLE IF NOT EXISTS agent_runs (
    id TEXT PRIMARY KEY,
    workflow_id INTEGER,
    status TEXT DEFAULT 'running',
    max_cost_cents INTEGER DEFAULT 0,
    current_cost_cents INTEGER DEFAULT 0,
    timeline TEXT DEFAULT '[]',
    checkpoints TEXT DEFAULT '[]',
    awaiting_approval INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status);
`

func migrateAgent(conn *sql.DB) {
	conn.Exec(agentSchema)
}

func genAgentID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "ar_" + hex.EncodeToString(b)
}

// registerAgentRoutes mounts agent runtime endpoints.
func registerAgentRoutes(mux *http.ServeMux, conn *sql.DB, audit func(string, string, string, string, any)) {
	mux.HandleFunc("POST /api/forge/agent-runs", handleCreateAgentRun(conn))
	mux.HandleFunc("GET /api/forge/runs/{id}/timeline", handleAgentTimeline(conn))
	mux.HandleFunc("POST /api/forge/runs/{id}/approve", handleAgentApprove(conn))
	mux.HandleFunc("POST /api/forge/runs/{id}/kill", handleAgentKill(conn, audit))
	mux.HandleFunc("POST /api/forge/runs/{id}/replay", handleAgentReplay(conn))
	mux.HandleFunc("GET /api/forge/agent-runs", handleListAgentRuns(conn))
	log.Printf("[forge] agent runtime routes registered")
}

func handleCreateAgentRun(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WorkflowID   int   `json:"workflow_id"`
			MaxCostCents int64 `json:"max_cost_cents"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		id := genAgentID()
		now := time.Now().UTC().Format(time.RFC3339)

		initialTimeline, _ := json.Marshal([]map[string]any{
			{"type": "start", "timestamp": now, "detail": "Agent run started"},
		})

		conn.Exec(`INSERT INTO agent_runs (id, workflow_id, status, max_cost_cents, timeline, created_at) VALUES (?, ?, 'running', ?, ?, ?)`,
			id, req.WorkflowID, req.MaxCostCents, string(initialTimeline), now)

		writeAgentJSON(w, http.StatusCreated, map[string]any{
			"id":             id,
			"status":         "running",
			"max_cost_cents": req.MaxCostCents,
		})
	}
}

func handleAgentTimeline(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var status, timelineJSON, checkpointsJSON, createdAt string
		var completedAt sql.NullString
		var maxCost, currentCost int64

		err := conn.QueryRow(`SELECT status, max_cost_cents, current_cost_cents, timeline, checkpoints, created_at, completed_at
			FROM agent_runs WHERE id = ?`, id).
			Scan(&status, &maxCost, &currentCost, &timelineJSON, &checkpointsJSON, &createdAt, &completedAt)
		if err != nil {
			writeAgentJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
			return
		}

		var timeline, checkpoints any
		if err := json.Unmarshal([]byte(timelineJSON), &timeline); err != nil {
			log.Printf("[agent] json parse error: %v", err)
		}
		if err := json.Unmarshal([]byte(checkpointsJSON), &checkpoints); err != nil {
			log.Printf("[agent] json parse error: %v", err)
		}

		writeAgentJSON(w, http.StatusOK, map[string]any{
			"id": id, "status": status,
			"max_cost_cents":     maxCost,
			"current_cost_cents": currentCost,
			"timeline":           timeline,
			"checkpoints":        checkpoints,
			"created_at":         createdAt,
			"completed_at":       completedAt.String,
		})
	}
}

func handleAgentApprove(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// Check if the run is awaiting approval.
		var awaiting int
		conn.QueryRow("SELECT awaiting_approval FROM agent_runs WHERE id = ?", id).Scan(&awaiting)
		if awaiting != 1 {
			writeAgentJSON(w, http.StatusBadRequest, map[string]string{"error": "run is not awaiting approval"})
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		conn.Exec("UPDATE agent_runs SET awaiting_approval = 0, status = 'running' WHERE id = ?", id)

		// Append to timeline.
		appendTimeline(conn, id, "checkpoint_approved", now, "Human approved checkpoint")

		writeAgentJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})
	}
}

func handleAgentKill(conn *sql.DB, audit func(string, string, string, string, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		now := time.Now().UTC().Format(time.RFC3339)
		result, _ := conn.Exec("UPDATE agent_runs SET status = 'killed', completed_at = ? WHERE id = ? AND status IN ('running', 'paused')", now, id)
		n, _ := result.RowsAffected()
		if n == 0 {
			writeAgentJSON(w, http.StatusNotFound, map[string]string{"error": "run not found or already completed"})
			return
		}

		appendTimeline(conn, id, "killed", now, "Agent run killed by operator")

		if audit != nil {
			audit("forge", "admin", "agent_run", "kill", map[string]any{"run_id": id})
		}

		writeAgentJSON(w, http.StatusOK, map[string]string{"status": "killed", "id": id})
	}
}

func handleAgentReplay(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// Get the original run.
		var workflowID int
		var maxCost int64
		err := conn.QueryRow("SELECT workflow_id, max_cost_cents FROM agent_runs WHERE id = ?", id).Scan(&workflowID, &maxCost)
		if err != nil {
			writeAgentJSON(w, http.StatusNotFound, map[string]string{"error": "original run not found"})
			return
		}

		// Allow overrides.
		var req struct {
			MaxCostCents *int64 `json:"max_cost_cents"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.MaxCostCents != nil {
			maxCost = *req.MaxCostCents
		}

		// Create a new run as a replay.
		newID := genAgentID()
		now := time.Now().UTC().Format(time.RFC3339)
		initialTimeline, _ := json.Marshal([]map[string]any{
			{"type": "replay_start", "timestamp": now, "detail": "Replay of " + id},
		})

		conn.Exec(`INSERT INTO agent_runs (id, workflow_id, status, max_cost_cents, timeline, created_at) VALUES (?, ?, 'running', ?, ?, ?)`,
			newID, workflowID, maxCost, string(initialTimeline), now)

		writeAgentJSON(w, http.StatusCreated, map[string]any{
			"id":             newID,
			"replayed_from":  id,
			"status":         "running",
			"max_cost_cents": maxCost,
		})
	}
}

func handleListAgentRuns(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`SELECT id, workflow_id, status, max_cost_cents, current_cost_cents, awaiting_approval, created_at, completed_at
			FROM agent_runs ORDER BY created_at DESC LIMIT 50`)
		if err != nil {
			writeAgentJSON(w, http.StatusOK, map[string]any{"runs": []any{}})
			return
		}
		defer rows.Close()

		var runs []map[string]any
		for rows.Next() {
			var id, status, createdAt string
			var completedAt sql.NullString
			var workflowID int
			var maxCost, currentCost int64
			var awaiting int
			if err := rows.Scan(&id, &workflowID, &status, &maxCost, &currentCost, &awaiting, &createdAt, &completedAt); err != nil {
				continue
			}
			runs = append(runs, map[string]any{
				"id": id, "workflow_id": workflowID, "status": status,
				"max_cost_cents": maxCost, "current_cost_cents": currentCost,
				"awaiting_approval": awaiting == 1,
				"created_at":        createdAt, "completed_at": completedAt.String,
			})
		}
		if runs == nil {
			runs = []map[string]any{}
		}
		writeAgentJSON(w, http.StatusOK, map[string]any{"runs": runs, "count": len(runs)})
	}
}

func appendTimeline(conn *sql.DB, runID, eventType, timestamp, detail string) {
	var timelineJSON string
	conn.QueryRow("SELECT timeline FROM agent_runs WHERE id = ?", runID).Scan(&timelineJSON)
	var timeline []map[string]any
	if err := json.Unmarshal([]byte(timelineJSON), &timeline); err != nil {
		log.Printf("[agent] json parse error: %v", err)
	}
	timeline = append(timeline, map[string]any{
		"type": eventType, "timestamp": timestamp, "detail": detail,
	})
	updated, _ := json.Marshal(timeline)
	conn.Exec("UPDATE agent_runs SET timeline = ? WHERE id = ?", string(updated), runID)
}

func writeAgentJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
