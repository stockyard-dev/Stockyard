package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Insight struct {
	ID          string    `json:"id"`
	InsightType string    `json:"insight_type"`
	Model       string    `json:"model"`
	Provider    string    `json:"provider"`
	Summary     string    `json:"summary"`
	Data        string    `json:"data"`
	Confidence  float64   `json:"confidence"`
	SampleSize  int       `json:"sample_size"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
}

type Peer struct {
	ID               string    `json:"id"`
	Endpoint         string    `json:"endpoint"`
	InstanceID       string    `json:"instance_id"`
	LastExchange     *time.Time `json:"last_exchange"`
	InsightsReceived int       `json:"insights_received"`
	InsightsSent     int       `json:"insights_sent"`
	TrustScore       float64   `json:"trust_score"`
	Status           string    `json:"status"`
}

type Exchange struct {
	ID           string    `json:"id"`
	PeerID       string    `json:"peer_id"`
	Direction    string    `json:"direction"`
	InsightCount int       `json:"insight_count"`
	ExchangedAt  time.Time `json:"exchanged_at"`
}

func Open(path string) (*DB, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, fmt.Errorf("open sqlite: %w", err) }
	conn.SetMaxOpenConns(4); conn.SetMaxIdleConns(2)
	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil { conn.Close(); return nil, fmt.Errorf("migrate: %w", err) }
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS mycelium_insights (
			id TEXT PRIMARY KEY, insight_type TEXT NOT NULL, model TEXT DEFAULT '',
			provider TEXT DEFAULT '', summary TEXT NOT NULL, data TEXT NOT NULL,
			confidence REAL DEFAULT 0, sample_size INTEGER DEFAULT 0,
			source TEXT DEFAULT 'local', created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_mycelium_type ON mycelium_insights(insight_type);
		CREATE INDEX IF NOT EXISTS idx_mycelium_model ON mycelium_insights(model, provider);
		CREATE TABLE IF NOT EXISTS mycelium_peers (
			id TEXT PRIMARY KEY, endpoint TEXT NOT NULL, instance_id TEXT NOT NULL,
			last_exchange DATETIME, insights_received INTEGER DEFAULT 0,
			insights_sent INTEGER DEFAULT 0, trust_score REAL DEFAULT 0.5,
			status TEXT DEFAULT 'discovered'
		);
		CREATE TABLE IF NOT EXISTS mycelium_exchanges (
			id TEXT PRIMARY KEY, peer_id TEXT NOT NULL, direction TEXT NOT NULL,
			insight_count INTEGER NOT NULL, exchanged_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreateInsight(i *Insight) error {
	if i.Source == "" { i.Source = "local" }
	_, err := db.conn.Exec(`INSERT INTO mycelium_insights (id,insight_type,model,provider,summary,data,confidence,sample_size,source) VALUES (?,?,?,?,?,?,?,?,?)`,
		i.ID, i.InsightType, i.Model, i.Provider, i.Summary, i.Data, i.Confidence, i.SampleSize, i.Source)
	return err
}

func (db *DB) ListInsights(insightType, model string, limit int) ([]Insight, error) {
	if limit <= 0 { limit = 50 }
	q := `SELECT id,insight_type,model,provider,summary,data,confidence,sample_size,source,created_at FROM mycelium_insights WHERE 1=1`
	var args []any
	if insightType != "" { q += " AND insight_type=?"; args = append(args, insightType) }
	if model != "" { q += " AND model=?"; args = append(args, model) }
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Insight
	for rows.Next() {
		var i Insight
		if err := rows.Scan(&i.ID, &i.InsightType, &i.Model, &i.Provider, &i.Summary, &i.Data, &i.Confidence, &i.SampleSize, &i.Source, &i.CreatedAt); err != nil {
			continue
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return out, nil
}

func (db *DB) CreatePeer(p *Peer) error {
	_, err := db.conn.Exec(`INSERT INTO mycelium_peers (id,endpoint,instance_id,trust_score,status) VALUES (?,?,?,?,?)`,
		p.ID, p.Endpoint, p.InstanceID, p.TrustScore, p.Status)
	return err
}

func (db *DB) ListPeers() ([]Peer, error) {
	rows, err := db.conn.Query(`SELECT id,endpoint,instance_id,last_exchange,insights_received,insights_sent,trust_score,status FROM mycelium_peers ORDER BY trust_score DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.ID, &p.Endpoint, &p.InstanceID, &p.LastExchange, &p.InsightsReceived, &p.InsightsSent, &p.TrustScore, &p.Status); err != nil {
			continue
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var insights, peers, exchanges int
	db.conn.QueryRow(`SELECT COUNT(*) FROM mycelium_insights`).Scan(&insights)
	db.conn.QueryRow(`SELECT COUNT(*) FROM mycelium_peers`).Scan(&peers)
	db.conn.QueryRow(`SELECT COUNT(*) FROM mycelium_exchanges`).Scan(&exchanges)
	return map[string]any{"total_insights": insights, "connected_peers": peers, "total_exchanges": exchanges}, nil
}

func (db *DB) CreateExchange(e *Exchange) error {
	_, err := db.conn.Exec(`INSERT INTO mycelium_exchanges (id,peer_id,direction,insight_count) VALUES (?,?,?,?)`,
		e.ID, e.PeerID, e.Direction, e.InsightCount)
	return err
}

func (db *DB) UpdatePeerExchange(id string, sent, received int) error {
	_, err := db.conn.Exec(`UPDATE mycelium_peers SET last_exchange=CURRENT_TIMESTAMP, insights_sent=insights_sent+?, insights_received=insights_received+?, status='connected' WHERE id=?`,
		sent, received, id)
	return err
}

func (db *DB) GetPeer(id string) (*Peer, error) {
	var p Peer
	err := db.conn.QueryRow(`SELECT id,endpoint,instance_id,last_exchange,insights_received,insights_sent,trust_score,status FROM mycelium_peers WHERE id=?`, id).Scan(
		&p.ID, &p.Endpoint, &p.InstanceID, &p.LastExchange, &p.InsightsReceived, &p.InsightsSent, &p.TrustScore, &p.Status)
	if err != nil { return nil, err }
	return &p, nil
}
