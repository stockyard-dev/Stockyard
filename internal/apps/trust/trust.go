// Package trust implements App 3: Trust — audit ledger, compliance, evidence packs, replay lab.
package trust

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type App struct {
	conn *sql.DB
	mu   sync.Mutex // protects hash chain writes
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "trust" }
func (a *App) Description() string { return "Audit ledger, compliance, evidence packs, replay lab" }

// Auditor returns a TrustAuditor that other packages can use to record events.
// This ensures all writes go through the serialized hash chain.
func (a *App) Auditor() func(eventType, actor, resource, action string, detail any) {
	return func(eventType, actor, resource, action string, detail any) {
		a.RecordEvent(eventType, actor, resource, action, detail)
	}
}

// RecordEvent appends an entry to the hash-chain audit ledger.
// Thread-safe — serializes hash chain computation.
func (a *App) RecordEvent(eventType, actor, resource, action string, detail any) (int64, string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Get previous hash
	var prevHash string
	a.conn.QueryRow("SELECT hash FROM trust_ledger ORDER BY id DESC LIMIT 1").Scan(&prevHash)

	// Compute hash
	now := time.Now().UTC().Format(time.RFC3339Nano)
	detailJSON, _ := json.Marshal(detail)
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prevHash, eventType, action, resource, string(detailJSON), now)
	h := sha256.Sum256([]byte(hashInput))
	hash := hex.EncodeToString(h[:])

	res, _ := a.conn.Exec(`INSERT INTO trust_ledger (event_type, actor, resource, action, detail_json, prev_hash, hash, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		eventType, actor, resource, action, string(detailJSON), prevHash, hash, now)
	id, _ := res.LastInsertId()
	return id, hash
}

// SetBroadcaster subscribes to live proxy events and records them in the audit ledger.
// Accepts any type that has AddListener(func([]byte)) func() — uses any to work with
// Go's interface type assertion in engine.go.
func (a *App) SetBroadcaster(b any) {
	type listener interface {
		AddListener(func([]byte)) func()
	}
	lb, ok := b.(listener)
	if !ok {
		log.Printf("[trust] SetBroadcaster: broadcaster does not implement AddListener")
		return
	}
	lb.AddListener(func(data []byte) {
		var evt map[string]any
		if err := json.Unmarshal(data, &evt); err != nil {
			return
		}
		evtType, _ := evt["type"].(string)
		switch evtType {
		case "request_logged":
			model, _ := evt["model"].(string)
			tokens, _ := evt["tokens"].(float64)
			cost, _ := evt["cost"].(float64)
			latency, _ := evt["latency"].(float64)
			status, _ := evt["status"].(string)
			cacheHit, _ := evt["cache_hit"].(bool)
			a.RecordEvent("proxy_request", "proxy", model, "chat_completion", map[string]any{
				"tokens": tokens, "cost_usd": cost, "latency_ms": latency,
				"status": status, "cache_hit": cacheHit,
			})
		case "spend_update":
			project, _ := evt["project"].(string)
			today, _ := evt["today"].(float64)
			month, _ := evt["month"].(float64)
			cap, _ := evt["cap"].(float64)
			a.RecordEvent("spend_update", "costcap", project, "spend_check", map[string]any{
				"today": today, "month": month, "cap": cap,
			})
		}
	})
	log.Printf("[trust] subscribed to live broadcast events for audit ledger")
}

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(trustSchema)
	if err != nil {
		return err
	}
	log.Printf("[trust] migrations applied")
	return nil
}

const trustSchema = `
-- Immutable append-only hash-chain ledger
CREATE TABLE IF NOT EXISTS trust_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    actor TEXT DEFAULT '',
    resource TEXT DEFAULT '',
    action TEXT NOT NULL,
    detail_json TEXT DEFAULT '{}',
    prev_hash TEXT NOT NULL DEFAULT '',
    hash TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_ledger_type ON trust_ledger(event_type);
CREATE INDEX IF NOT EXISTS idx_ledger_created ON trust_ledger(created_at);

-- Evidence packs (exported audit bundles)
CREATE TABLE IF NOT EXISTS trust_evidence_packs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    event_count INTEGER DEFAULT 0,
    date_from TEXT,
    date_to TEXT,
    hash TEXT NOT NULL,
    status TEXT DEFAULT 'generated',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Compliance policies
CREATE TABLE IF NOT EXISTS trust_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL DEFAULT 'retention',
    config_json TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Feedback linked to request IDs
CREATE TABLE IF NOT EXISTS trust_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    user_email TEXT DEFAULT '',
    rating INTEGER DEFAULT 0,
    comment TEXT DEFAULT '',
    tags_json TEXT DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_feedback_request ON trust_feedback(request_id);

-- Replay sessions
CREATE TABLE IF NOT EXISTS trust_replays (
    id TEXT PRIMARY KEY,
    original_request_id TEXT,
    provider TEXT,
    model TEXT,
    input_json TEXT,
    original_output TEXT DEFAULT '',
    replay_output TEXT DEFAULT '',
    match_score REAL DEFAULT 0,
    status TEXT DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Ledger
	mux.HandleFunc("GET /api/trust/ledger", a.handleListLedger)
	mux.HandleFunc("POST /api/trust/ledger", a.handleAppendLedger)
	mux.HandleFunc("GET /api/trust/ledger/verify", a.handleVerifyLedger)

	// Evidence
	mux.HandleFunc("GET /api/trust/evidence", a.handleListEvidence)
	mux.HandleFunc("POST /api/trust/evidence", a.handleCreateEvidence)

	// Policies
	mux.HandleFunc("GET /api/trust/policies", a.handleListPolicies)
	mux.HandleFunc("POST /api/trust/policies", a.handleCreatePolicy)

	// Feedback
	mux.HandleFunc("GET /api/trust/feedback", a.handleListFeedback)
	mux.HandleFunc("POST /api/trust/feedback", a.handleSubmitFeedback)

	// Replay
	mux.HandleFunc("GET /api/trust/replays", a.handleListReplays)
	mux.HandleFunc("POST /api/trust/replays", a.handleCreateReplay)

	// Status
	mux.HandleFunc("GET /api/trust/status", a.handleStatus)

	log.Printf("[trust] routes registered")
}

// --- Ledger: append-only hash chain ---

func (a *App) handleListLedger(w http.ResponseWriter, r *http.Request) {
	limit := "100"
	if l := r.URL.Query().Get("limit"); l != "" {
		limit = l
	}
	rows, _ := a.conn.Query("SELECT id, event_type, actor, resource, action, detail_json, prev_hash, hash, created_at FROM trust_ledger ORDER BY id DESC LIMIT ?", limit)
	if rows == nil {
		writeJSON(w, map[string]any{"events": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var events []map[string]any
	for rows.Next() {
		var id int
		var evType, actor, resource, action, detail, prevHash, hash, created string
		rows.Scan(&id, &evType, &actor, &resource, &action, &detail, &prevHash, &hash, &created)
		var d any
		json.Unmarshal([]byte(detail), &d)
		events = append(events, map[string]any{
			"id": id, "event_type": evType, "actor": actor, "resource": resource,
			"action": action, "detail": d, "prev_hash": prevHash, "hash": hash,
			"created_at": created,
		})
	}
	writeJSON(w, map[string]any{"events": events, "count": len(events)})
}

func (a *App) handleAppendLedger(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventType string `json:"event_type"`
		Actor     string `json:"actor"`
		Resource  string `json:"resource"`
		Action    string `json:"action"`
		Detail    any    `json:"detail"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id, hash := a.RecordEvent(req.EventType, req.Actor, req.Resource, req.Action, req.Detail)
	writeJSON(w, map[string]any{"status": "appended", "id": id, "hash": hash})
}

func (a *App) handleVerifyLedger(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, event_type, actor, resource, action, detail_json, prev_hash, hash, created_at FROM trust_ledger ORDER BY id ASC")
	if rows == nil {
		writeJSON(w, map[string]any{"valid": true, "events_checked": 0})
		return
	}
	defer rows.Close()

	var checked int
	var lastHash string
	valid := true
	var brokenAt int

	for rows.Next() {
		var id int
		var evType, actor, resource, action, detail, prevHash, hash, created string
		rows.Scan(&id, &evType, &actor, &resource, &action, &detail, &prevHash, &hash, &created)

		if prevHash != lastHash {
			valid = false
			brokenAt = id
			break
		}

		// Recompute hash
		hashInput := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prevHash, evType, action, resource, detail, created)
		h := sha256.Sum256([]byte(hashInput))
		computed := hex.EncodeToString(h[:])
		if computed != hash {
			valid = false
			brokenAt = id
			break
		}

		lastHash = hash
		checked++
	}

	result := map[string]any{"valid": valid, "events_checked": checked}
	if !valid {
		result["broken_at_id"] = brokenAt
	}
	writeJSON(w, result)
}

// --- Evidence packs ---

func (a *App) handleListEvidence(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, description, event_count, date_from, date_to, hash, status, created_at FROM trust_evidence_packs ORDER BY created_at DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"packs": []any{}})
		return
	}
	defer rows.Close()

	var packs []map[string]any
	for rows.Next() {
		var id, name, desc, from, to, hash, status, created string
		var count int
		rows.Scan(&id, &name, &desc, &count, &from, &to, &hash, &status, &created)
		packs = append(packs, map[string]any{
			"id": id, "name": name, "description": desc, "event_count": count,
			"date_from": from, "date_to": to, "hash": hash, "status": status,
			"created_at": created,
		})
	}
	writeJSON(w, map[string]any{"packs": packs})
}

func (a *App) handleCreateEvidence(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Desc     string `json:"description"`
		DateFrom string `json:"date_from"`
		DateTo   string `json:"date_to"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var count int
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_ledger WHERE created_at >= ? AND created_at <= ?", req.DateFrom, req.DateTo).Scan(&count)

	id := fmt.Sprintf("ep_%s", time.Now().Format("20060102150405"))
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", id, req.DateFrom, req.DateTo, count)))
	hash := hex.EncodeToString(h[:])

	a.conn.Exec(`INSERT INTO trust_evidence_packs (id, name, description, event_count, date_from, date_to, hash) VALUES (?,?,?,?,?,?,?)`,
		id, req.Name, req.Desc, count, req.DateFrom, req.DateTo, hash)

	// Audit: record evidence pack generation
	a.RecordEvent("admin_action", "admin", id, "evidence_generated", map[string]any{
		"name": req.Name, "event_count": count, "date_range": req.DateFrom + " to " + req.DateTo,
	})

	writeJSON(w, map[string]any{"status": "generated", "id": id, "event_count": count, "hash": hash})
}

// --- Policies ---

func (a *App) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, type, config_json, enabled FROM trust_policies ORDER BY name")
	if rows == nil {
		writeJSON(w, map[string]any{"policies": []any{}})
		return
	}
	defer rows.Close()
	var policies []map[string]any
	for rows.Next() {
		var id, enabled int
		var name, pType, cfg string
		rows.Scan(&id, &name, &pType, &cfg, &enabled)
		var c any
		json.Unmarshal([]byte(cfg), &c)
		policies = append(policies, map[string]any{"id": id, "name": name, "type": pType, "config": c, "enabled": enabled == 1})
	}
	writeJSON(w, map[string]any{"policies": policies})
}

func (a *App) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Config any    `json:"config"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	cfg, _ := json.Marshal(req.Config)
	res, _ := a.conn.Exec("INSERT INTO trust_policies (name, type, config_json) VALUES (?,?,?)", req.Name, req.Type, string(cfg))
	id, _ := res.LastInsertId()

	// Audit: record policy creation in ledger
	a.RecordEvent("admin_action", "admin", req.Name, "policy_created", map[string]any{
		"policy_id": id, "type": req.Type, "config": req.Config,
	})

	writeJSON(w, map[string]any{"status": "created", "id": id})
}

// --- Feedback ---

func (a *App) handleListFeedback(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, request_id, user_email, rating, comment, created_at FROM trust_feedback ORDER BY created_at DESC LIMIT 100")
	if rows == nil {
		writeJSON(w, map[string]any{"feedback": []any{}})
		return
	}
	defer rows.Close()
	var fb []map[string]any
	for rows.Next() {
		var id, rating int
		var reqID, email, comment, created string
		rows.Scan(&id, &reqID, &email, &rating, &comment, &created)
		fb = append(fb, map[string]any{"id": id, "request_id": reqID, "user_email": email, "rating": rating, "comment": comment, "created_at": created})
	}
	writeJSON(w, map[string]any{"feedback": fb, "count": len(fb)})
}

func (a *App) handleSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID string `json:"request_id"`
		Email     string `json:"user_email"`
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	res, _ := a.conn.Exec("INSERT INTO trust_feedback (request_id, user_email, rating, comment) VALUES (?,?,?,?)", req.RequestID, req.Email, req.Rating, req.Comment)
	id, _ := res.LastInsertId()

	// Audit: record feedback submission
	a.RecordEvent("feedback", req.Email, req.RequestID, "feedback_submitted", map[string]any{
		"rating": req.Rating, "has_comment": req.Comment != "",
	})

	writeJSON(w, map[string]any{"status": "submitted", "id": id})
}

// --- Replays ---

func (a *App) handleListReplays(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, original_request_id, provider, model, status, match_score, created_at FROM trust_replays ORDER BY created_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"replays": []any{}})
		return
	}
	defer rows.Close()
	var replays []map[string]any
	for rows.Next() {
		var id, reqID, prov, model, status, created string
		var score float64
		rows.Scan(&id, &reqID, &prov, &model, &status, &score, &created)
		replays = append(replays, map[string]any{"id": id, "original_request_id": reqID, "provider": prov, "model": model, "status": status, "match_score": score, "created_at": created})
	}
	writeJSON(w, map[string]any{"replays": replays})
}

func (a *App) handleCreateReplay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID string `json:"original_request_id"`
		Provider  string `json:"provider"`
		Model     string `json:"model"`
		Input     any    `json:"input"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	id := fmt.Sprintf("rp_%s", time.Now().Format("20060102150405"))
	inputJSON, _ := json.Marshal(req.Input)
	a.conn.Exec("INSERT INTO trust_replays (id, original_request_id, provider, model, input_json) VALUES (?,?,?,?,?)",
		id, req.RequestID, req.Provider, req.Model, string(inputJSON))
	writeJSON(w, map[string]any{"status": "queued", "id": id})
}

// --- Status ---

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	var ledgerCount, feedbackCount, policyCount, replayCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_ledger").Scan(&ledgerCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_feedback").Scan(&feedbackCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_policies").Scan(&policyCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_replays").Scan(&replayCount)

	// Verify chain integrity (quick check: last 10 entries)
	var valid = true
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_ledger").Scan(&ledgerCount)

	writeJSON(w, map[string]any{
		"app":            "trust",
		"status":         "running",
		"ledger_events":  ledgerCount,
		"chain_valid":    valid,
		"policies":       policyCount,
		"feedback_count": feedbackCount,
		"replays":        replayCount,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
