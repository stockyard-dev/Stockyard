// Package mesh implements a multi-region proxy mesh for Stockyard.
// Nodes register with a control plane and sync configuration.
package mesh

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const meshSchema = `
CREATE TABLE IF NOT EXISTS mesh_nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT 'default',
    status TEXT DEFAULT 'healthy',
    last_heartbeat TEXT,
    metadata TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_mesh_nodes_region ON mesh_nodes(region);
CREATE INDEX IF NOT EXISTS idx_mesh_nodes_status ON mesh_nodes(status);

CREATE TABLE IF NOT EXISTS mesh_pricing (
    region TEXT PRIMARY KEY,
    markup_pct REAL DEFAULT 0,
    description TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mesh_transactions (
    id TEXT PRIMARY KEY,
    demand_node TEXT DEFAULT '',
    supply_node TEXT DEFAULT '',
    model TEXT DEFAULT '',
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    verified INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_mesh_tx_supply ON mesh_transactions(supply_node);

CREATE TABLE IF NOT EXISTS mesh_earnings (
    operator_id TEXT NOT NULL,
    period TEXT NOT NULL,
    tokens_served INTEGER DEFAULT 0,
    earnings_cents INTEGER DEFAULT 0,
    fee_cents INTEGER DEFAULT 0,
    PRIMARY KEY(operator_id, period)
);

CREATE TABLE IF NOT EXISTS mesh_verifications (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    passed INTEGER DEFAULT 1,
    similarity REAL DEFAULT 1.0,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS mesh_dedup_cache (
    prompt_hash TEXT PRIMARY KEY,
    response TEXT DEFAULT '',
    node_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT
);
`

// Manager handles mesh node registration and health tracking.
type Manager struct {
	conn *sql.DB
}

// NewManager creates a mesh manager.
func NewManager(conn *sql.DB) *Manager {
	conn.Exec(meshSchema)
	return &Manager{conn: conn}
}

func genNodeID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "mn_" + hex.EncodeToString(b)
}

// Register mounts mesh API routes.
func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mesh/register", m.handleRegister)
	mux.HandleFunc("GET /api/mesh/nodes", m.handleListNodes)
	mux.HandleFunc("POST /api/mesh/heartbeat", m.handleHeartbeat)
	mux.HandleFunc("GET /api/mesh/config", m.handleGetConfig)
	mux.HandleFunc("GET /api/mesh/pricing", m.handlePricing)
	mux.HandleFunc("GET /api/mesh/nodes/{id}", m.handleGetNode)
	mux.HandleFunc("POST /api/mesh/deregister", m.handleDeregister)
	mux.HandleFunc("GET /api/mesh/stats", m.handleStats)
	mux.HandleFunc("GET /api/mesh/earnings", m.handleEarnings)
	mux.HandleFunc("GET /api/mesh/earnings/summary", m.handleEarningsSummary)
	mux.HandleFunc("POST /api/mesh/payout", m.handlePayout)
	log.Printf("[mesh] routes registered")
}

func (m *Manager) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Region   string `json:"region"`
		Metadata any    `json:"metadata"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" || req.URL == "" {
		w.WriteHeader(400)
		writeMeshJSON(w, map[string]string{"error": "name and url required"})
		return
	}
	if req.Region == "" {
		req.Region = "default"
	}

	id := genNodeID()
	now := time.Now().UTC().Format(time.RFC3339)
	metaJSON, _ := json.Marshal(req.Metadata)

	m.conn.Exec(`INSERT OR REPLACE INTO mesh_nodes (id, name, url, region, status, last_heartbeat, metadata, created_at)
		VALUES (?, ?, ?, ?, 'healthy', ?, ?, ?)`,
		id, req.Name, req.URL, req.Region, now, string(metaJSON), now)

	writeMeshJSON(w, map[string]any{
		"id": id, "name": req.Name, "region": req.Region, "status": "registered",
	})
}

func (m *Manager) handleListNodes(w http.ResponseWriter, r *http.Request) {
	rows, err := m.conn.Query(`SELECT id, name, url, region, status, last_heartbeat, metadata, created_at
		FROM mesh_nodes ORDER BY region, name`)
	if err != nil {
		writeMeshJSON(w, map[string]any{"nodes": []any{}})
		return
	}
	defer rows.Close()

	var nodes []map[string]any
	for rows.Next() {
		var id, name, url, region, status, heartbeat, metaJSON, createdAt string
		rows.Scan(&id, &name, &url, &region, &status, &heartbeat, &metaJSON, &createdAt)
		var meta any
		json.Unmarshal([]byte(metaJSON), &meta)

		// Mark nodes as unhealthy if no heartbeat in 2 minutes.
		if heartbeat != "" {
			t, _ := time.Parse(time.RFC3339, heartbeat)
			if time.Since(t) > 2*time.Minute {
				status = "unhealthy"
				m.conn.Exec("UPDATE mesh_nodes SET status = 'unhealthy' WHERE id = ?", id)
			}
		}

		nodes = append(nodes, map[string]any{
			"id": id, "name": name, "url": url, "region": region,
			"status": status, "last_heartbeat": heartbeat, "metadata": meta,
			"created_at": createdAt,
		})
	}
	if nodes == nil {
		nodes = []map[string]any{}
	}

	// Group by region.
	byRegion := make(map[string]int)
	for _, n := range nodes {
		byRegion[n["region"].(string)]++
	}

	writeMeshJSON(w, map[string]any{
		"nodes":      nodes,
		"count":      len(nodes),
		"by_region":  byRegion,
	})
}

func (m *Manager) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID string `json:"node_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.NodeID == "" {
		w.WriteHeader(400)
		writeMeshJSON(w, map[string]string{"error": "node_id required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	m.conn.Exec("UPDATE mesh_nodes SET status = 'healthy', last_heartbeat = ? WHERE id = ?", now, req.NodeID)
	writeMeshJSON(w, map[string]string{"status": "ok"})
}

func (m *Manager) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return current config snapshot for node sync.
	var modules []map[string]any
	rows, err := m.conn.Query("SELECT name, enabled FROM proxy_modules ORDER BY name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var enabled int
			rows.Scan(&name, &enabled)
			modules = append(modules, map[string]any{"name": name, "enabled": enabled == 1})
		}
	}
	if modules == nil {
		modules = []map[string]any{}
	}

	writeMeshJSON(w, map[string]any{
		"modules":    modules,
		"synced_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (m *Manager) handlePricing(w http.ResponseWriter, r *http.Request) {
	rows, err := m.conn.Query("SELECT region, markup_pct, description FROM mesh_pricing ORDER BY region")
	if err != nil {
		writeMeshJSON(w, map[string]any{"pricing": []any{}})
		return
	}
	defer rows.Close()

	var pricing []map[string]any
	for rows.Next() {
		var region, desc string
		var markup float64
		rows.Scan(&region, &markup, &desc)
		pricing = append(pricing, map[string]any{
			"region": region, "markup_pct": markup, "description": desc,
		})
	}
	if pricing == nil {
		pricing = []map[string]any{}
	}
	writeMeshJSON(w, map[string]any{"pricing": pricing})
}

func (m *Manager) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var name, url, region, status, heartbeat, metaJSON, createdAt string
	err := m.conn.QueryRow(`SELECT name, url, region, status, last_heartbeat, metadata, created_at
		FROM mesh_nodes WHERE id = ?`, id).Scan(&name, &url, &region, &status, &heartbeat, &metaJSON, &createdAt)
	if err != nil {
		w.WriteHeader(404)
		writeMeshJSON(w, map[string]string{"error": "node not found"})
		return
	}
	var meta any
	json.Unmarshal([]byte(metaJSON), &meta)
	writeMeshJSON(w, map[string]any{
		"id": id, "name": name, "url": url, "region": region,
		"status": status, "last_heartbeat": heartbeat, "metadata": meta, "created_at": createdAt,
	})
}

func (m *Manager) handleDeregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID string `json:"node_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.NodeID == "" {
		w.WriteHeader(400)
		writeMeshJSON(w, map[string]string{"error": "node_id required"})
		return
	}
	m.conn.Exec(`UPDATE mesh_nodes SET status = 'offline' WHERE id = ?`, req.NodeID)
	writeMeshJSON(w, map[string]string{"status": "deregistered"})
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	var totalNodes, healthyNodes int
	m.conn.QueryRow(`SELECT COUNT(*) FROM mesh_nodes`).Scan(&totalNodes)
	m.conn.QueryRow(`SELECT COUNT(*) FROM mesh_nodes WHERE status = 'healthy'`).Scan(&healthyNodes)
	var totalTx, totalTokens, totalCost int
	m.conn.QueryRow(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(input_tokens+output_tokens),0), COALESCE(SUM(cost_cents),0) FROM mesh_transactions`).Scan(&totalTx, &totalTokens, &totalCost)
	writeMeshJSON(w, map[string]any{
		"total_nodes": totalNodes, "healthy_nodes": healthyNodes,
		"total_transactions": totalTx, "total_tokens_served": totalTokens, "total_cost_cents": totalCost,
	})
}

func (m *Manager) handleEarnings(w http.ResponseWriter, r *http.Request) {
	rows, err := m.conn.Query(`SELECT operator_id, period, tokens_served, earnings_cents, fee_cents
		FROM mesh_earnings ORDER BY period DESC LIMIT 50`)
	if err != nil {
		writeMeshJSON(w, map[string]any{"earnings": []any{}})
		return
	}
	defer rows.Close()
	var earnings []map[string]any
	for rows.Next() {
		var opID, period string
		var tokens, earn, fee int
		if rows.Scan(&opID, &period, &tokens, &earn, &fee) == nil {
			earnings = append(earnings, map[string]any{
				"operator_id": opID, "period": period, "tokens_served": tokens,
				"earnings_cents": earn, "fee_cents": fee, "net_cents": earn - fee,
			})
		}
	}
	if earnings == nil {
		earnings = []map[string]any{}
	}
	writeMeshJSON(w, map[string]any{"earnings": earnings})
}

func (m *Manager) handleEarningsSummary(w http.ResponseWriter, r *http.Request) {
	period := time.Now().UTC().Format("2006-01")
	var totalEarnings, totalFees, totalTokens int
	m.conn.QueryRow(`SELECT COALESCE(SUM(earnings_cents),0), COALESCE(SUM(fee_cents),0), COALESCE(SUM(tokens_served),0)
		FROM mesh_earnings WHERE period = ?`, period).Scan(&totalEarnings, &totalFees, &totalTokens)
	writeMeshJSON(w, map[string]any{
		"period": period, "total_earnings_cents": totalEarnings,
		"total_fees_cents": totalFees, "net_cents": totalEarnings - totalFees,
		"total_tokens_served": totalTokens,
	})
}

func (m *Manager) handlePayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OperatorID string `json:"operator_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.OperatorID == "" {
		w.WriteHeader(400)
		writeMeshJSON(w, map[string]string{"error": "operator_id required"})
		return
	}
	var net int
	m.conn.QueryRow(`SELECT COALESCE(SUM(earnings_cents - fee_cents), 0) FROM mesh_earnings WHERE operator_id = ?`,
		req.OperatorID).Scan(&net)
	writeMeshJSON(w, map[string]any{
		"operator_id": req.OperatorID, "payout_cents": net, "status": "pending",
	})
}

func writeMeshJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
