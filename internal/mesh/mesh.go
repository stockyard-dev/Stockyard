// Package mesh implements a multi-region proxy mesh for Stockyard.
// Nodes register with a control plane and sync configuration.
package mesh

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const meshSchema = `
CREATE TABLE IF NOT EXISTS mesh_nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT 'default',
    status TEXT DEFAULT 'healthy',
    gpu_model TEXT DEFAULT '',
    vram_gb INTEGER DEFAULT 0,
    supported_models TEXT DEFAULT '[]',
    current_load REAL DEFAULT 0,
    max_concurrent INTEGER DEFAULT 4,
    price_per_1k_cents INTEGER DEFAULT 1,
    reputation REAL DEFAULT 50,
    specialization TEXT DEFAULT '',
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
    demand_node TEXT,
    supply_node TEXT,
    model TEXT,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    verified INTEGER DEFAULT 0,
    created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_mesh_tx_supply ON mesh_transactions(supply_node);

CREATE TABLE IF NOT EXISTS mesh_earnings (
    operator_id TEXT,
    period TEXT,
    tokens_served INTEGER DEFAULT 0,
    earnings_cents INTEGER DEFAULT 0,
    fee_cents INTEGER DEFAULT 0,
    PRIMARY KEY(operator_id, period)
);

CREATE TABLE IF NOT EXISTS mesh_verifications (
    id TEXT PRIMARY KEY,
    node_id TEXT,
    passed INTEGER,
    similarity REAL,
    created_at TEXT
);
`

// Manager handles mesh node registration and health tracking.
type Manager struct {
	conn *sql.DB
}

// NewManager creates a mesh manager.
func NewManager(conn *sql.DB) (*Manager, error) {
	if _, err := conn.Exec(meshSchema); err != nil {
		return nil, fmt.Errorf("mesh schema: %w", err)
	}
	return &Manager{conn: conn}, nil
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
	mux.HandleFunc("GET /api/mesh/nodes/{id}", m.handleGetNode)
	mux.HandleFunc("POST /api/mesh/heartbeat", m.handleHeartbeat)
	mux.HandleFunc("POST /api/mesh/deregister", m.handleDeregister)
	mux.HandleFunc("GET /api/mesh/config", m.handleGetConfig)
	mux.HandleFunc("GET /api/mesh/pricing", m.handlePricing)
	mux.HandleFunc("GET /api/mesh/stats", m.handleStats)
	mux.HandleFunc("GET /api/mesh/earnings", m.handleEarnings)
	mux.HandleFunc("GET /api/mesh/earnings/summary", m.handleEarningsSummary)
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
	if err := validateNodeURL(req.URL); err != nil {
		w.WriteHeader(400)
		writeMeshJSON(w, map[string]string{"error": "invalid node URL: " + err.Error()})
		return
	}
	if req.Region == "" {
		req.Region = "default"
	}

	id := genNodeID()
	now := time.Now().UTC().Format(time.RFC3339)
	metaJSON, marshalErr := json.Marshal(req.Metadata)
	if marshalErr != nil {
		metaJSON = []byte("{}")
	}

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
		if err := rows.Scan(&id, &name, &url, &region, &status, &heartbeat, &metaJSON, &createdAt); err != nil {
			log.Printf("[mesh] scan node row: %v", err)
			continue
		}
		var meta any
		if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
			log.Printf("[mesh] failed to parse node metadata: %v", err)
		}

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
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
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
		"nodes":     nodes,
		"count":     len(nodes),
		"by_region": byRegion,
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
			if err := rows.Scan(&name, &enabled); err != nil {
				continue
			}
			modules = append(modules, map[string]any{"name": name, "enabled": enabled == 1})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}
	if modules == nil {
		modules = []map[string]any{}
	}

	writeMeshJSON(w, map[string]any{
		"modules":   modules,
		"synced_at": time.Now().UTC().Format(time.RFC3339),
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
		if err := rows.Scan(&region, &markup, &desc); err != nil {
			continue
		}
		pricing = append(pricing, map[string]any{
			"region": region, "markup_pct": markup, "description": desc,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if pricing == nil {
		pricing = []map[string]any{}
	}
	writeMeshJSON(w, map[string]any{"pricing": pricing})
}

func (m *Manager) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var name, url, region, status, gpuModel, supportedModels, specialization, heartbeat, metaJSON, createdAt string
	var vramGB, maxConcurrent, pricePer1k int
	var currentLoad, reputation float64
	err := m.conn.QueryRow(`SELECT name, url, region, status, gpu_model, vram_gb, supported_models,
		current_load, max_concurrent, price_per_1k_cents, reputation, specialization,
		last_heartbeat, metadata, created_at FROM mesh_nodes WHERE id = ?`, id).Scan(
		&name, &url, &region, &status, &gpuModel, &vramGB, &supportedModels,
		&currentLoad, &maxConcurrent, &pricePer1k, &reputation, &specialization,
		&heartbeat, &metaJSON, &createdAt)
	if err != nil {
		w.WriteHeader(404)
		writeMeshJSON(w, map[string]string{"error": "node not found"})
		return
	}
	var models []string
	if err := json.Unmarshal([]byte(supportedModels), &models); err != nil {
		log.Printf("[mesh] failed to parse supported models: %v", err)
	}
	var meta any
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		log.Printf("[mesh] failed to parse node metadata: %v", err)
	}
	writeMeshJSON(w, map[string]any{
		"id": id, "name": name, "url": url, "region": region, "status": status,
		"gpu_model": gpuModel, "vram_gb": vramGB, "supported_models": models,
		"current_load": currentLoad, "max_concurrent": maxConcurrent,
		"price_per_1k_cents": pricePer1k, "reputation": reputation,
		"specialization": specialization, "last_heartbeat": heartbeat,
		"metadata": meta, "created_at": createdAt,
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
	m.conn.Exec("UPDATE mesh_nodes SET status = 'offline' WHERE id = ?", req.NodeID)
	writeMeshJSON(w, map[string]string{"status": "deregistered", "node_id": req.NodeID})
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	var totalNodes, activeNodes, totalTx int
	var totalTokens, totalEarnings int
	m.conn.QueryRow("SELECT COUNT(*) FROM mesh_nodes").Scan(&totalNodes)
	m.conn.QueryRow("SELECT COUNT(*) FROM mesh_nodes WHERE status = 'healthy'").Scan(&activeNodes)
	m.conn.QueryRow("SELECT COUNT(*) FROM mesh_transactions").Scan(&totalTx)
	m.conn.QueryRow("SELECT COALESCE(SUM(input_tokens + output_tokens), 0) FROM mesh_transactions").Scan(&totalTokens)
	m.conn.QueryRow("SELECT COALESCE(SUM(earnings_cents), 0) FROM mesh_earnings").Scan(&totalEarnings)
	writeMeshJSON(w, map[string]any{
		"total_nodes": totalNodes, "active_nodes": activeNodes,
		"total_transactions": totalTx, "total_tokens_served": totalTokens,
		"total_earnings_cents": totalEarnings,
	})
}

func (m *Manager) handleEarnings(w http.ResponseWriter, r *http.Request) {
	operatorID := r.URL.Query().Get("operator_id")
	if operatorID == "" {
		operatorID = "default"
	}
	rows, err := m.conn.Query("SELECT period, tokens_served, earnings_cents, fee_cents FROM mesh_earnings WHERE operator_id = ? ORDER BY period DESC", operatorID)
	if err != nil {
		writeMeshJSON(w, map[string]any{"earnings": []any{}})
		return
	}
	defer rows.Close()
	var earnings []map[string]any
	for rows.Next() {
		var period string
		var tokens, earningsCents, feeCents int
		if err := rows.Scan(&period, &tokens, &earningsCents, &feeCents); err != nil {
			continue
		}
		earnings = append(earnings, map[string]any{
			"period": period, "tokens_served": tokens,
			"earnings_cents": earningsCents, "fee_cents": feeCents,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if earnings == nil {
		earnings = []map[string]any{}
	}
	writeMeshJSON(w, map[string]any{"earnings": earnings, "operator_id": operatorID})
}

func (m *Manager) handleEarningsSummary(w http.ResponseWriter, r *http.Request) {
	period := time.Now().UTC().Format("2006-01")
	var totalEarnings, totalFees, totalTokens int
	m.conn.QueryRow("SELECT COALESCE(SUM(earnings_cents), 0), COALESCE(SUM(fee_cents), 0), COALESCE(SUM(tokens_served), 0) FROM mesh_earnings WHERE period = ?", period).Scan(&totalEarnings, &totalFees, &totalTokens)
	writeMeshJSON(w, map[string]any{
		"period": period, "total_earnings_cents": totalEarnings,
		"total_fees_cents": totalFees, "total_tokens_served": totalTokens,
	})
}

// validateNodeURL checks that a mesh node URL is safe (no SSRF to internal services).
func validateNodeURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("blocked scheme %q (only http/https allowed)", scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must include a hostname")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		addrs, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("cannot resolve host %q: %w", host, err)
		}
		for _, addr := range addrs {
			if isPrivateMeshIP(addr) {
				return fmt.Errorf("blocked: %s resolves to private IP %s", host, addr)
			}
		}
		return nil
	}
	if isPrivateMeshIP(ip) {
		return fmt.Errorf("blocked: private/internal IP %s", ip)
	}
	return nil
}

func isPrivateMeshIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"127.0.0.0/8", "169.254.0.0/16",
		"::1/128", "fc00::/7", "fe80::/10",
	}
	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func writeMeshJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
