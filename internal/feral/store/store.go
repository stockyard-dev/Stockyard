package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Campaign struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	TargetURL        string    `json:"target_url"`
	AttackTypes      string    `json:"attack_types"`
	Generation       int       `json:"generation"`
	TotalAttacks     int       `json:"total_attacks"`
	SuccessfulAttacks int     `json:"successful_attacks"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type Attack struct {
	ID               string    `json:"id"`
	CampaignID       string    `json:"campaign_id"`
	Generation       int       `json:"generation"`
	ParentID         string    `json:"parent_id"`
	AttackType       string    `json:"attack_type"`
	Payload          string    `json:"payload"`
	Mutations        string    `json:"mutations"`
	BypassedGuardrail string  `json:"bypassed_guardrail"`
	Success          int       `json:"success"`
	ResponseSnippet  string    `json:"response_snippet"`
	CreatedAt        time.Time `json:"created_at"`
}

type Vulnerability struct {
	ID           string    `json:"id"`
	CampaignID   string    `json:"campaign_id"`
	Guardrail    string    `json:"guardrail"`
	AttackLineage string   `json:"attack_lineage"`
	CommonTrait  string    `json:"common_trait"`
	Severity     string    `json:"severity"`
	Patched      int       `json:"patched"`
	DiscoveredAt time.Time `json:"discovered_at"`
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
		CREATE TABLE IF NOT EXISTS feral_campaigns (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, target_url TEXT NOT NULL,
			attack_types TEXT DEFAULT '["injection","jailbreak","exfiltration","encoding"]',
			generation INTEGER DEFAULT 0, total_attacks INTEGER DEFAULT 0,
			successful_attacks INTEGER DEFAULT 0, status TEXT DEFAULT 'idle',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS feral_attacks (
			id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL, generation INTEGER NOT NULL,
			parent_id TEXT DEFAULT '', attack_type TEXT NOT NULL, payload TEXT NOT NULL,
			mutations TEXT DEFAULT '[]', bypassed_guardrail TEXT DEFAULT '',
			success INTEGER DEFAULT 0, response_snippet TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_feral_campaign ON feral_attacks(campaign_id, generation);
		CREATE INDEX IF NOT EXISTS idx_feral_success ON feral_attacks(success, campaign_id);
		CREATE TABLE IF NOT EXISTS feral_vulnerabilities (
			id TEXT PRIMARY KEY, campaign_id TEXT NOT NULL, guardrail TEXT NOT NULL,
			attack_lineage TEXT NOT NULL, common_trait TEXT NOT NULL,
			severity TEXT NOT NULL, patched INTEGER DEFAULT 0,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreateCampaign(c *Campaign) error {
	_, err := db.conn.Exec(`INSERT INTO feral_campaigns (id,name,target_url,attack_types) VALUES (?,?,?,?)`,
		c.ID, c.Name, c.TargetURL, c.AttackTypes)
	return err
}

func (db *DB) ListCampaigns() ([]Campaign, error) {
	rows, err := db.conn.Query(`SELECT id,name,target_url,attack_types,generation,total_attacks,successful_attacks,status,created_at FROM feral_campaigns ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Campaign
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.Name, &c.TargetURL, &c.AttackTypes, &c.Generation, &c.TotalAttacks, &c.SuccessfulAttacks, &c.Status, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return out, nil
}

func (db *DB) GetCampaign(id string) (*Campaign, error) {
	var c Campaign
	err := db.conn.QueryRow(`SELECT id,name,target_url,attack_types,generation,total_attacks,successful_attacks,status,created_at FROM feral_campaigns WHERE id=?`, id).Scan(
		&c.ID, &c.Name, &c.TargetURL, &c.AttackTypes, &c.Generation, &c.TotalAttacks, &c.SuccessfulAttacks, &c.Status, &c.CreatedAt)
	if err != nil { return nil, err }
	return &c, nil
}

func (db *DB) ListAttacks(campaignID string, successOnly bool, limit int) ([]Attack, error) {
	if limit <= 0 { limit = 50 }
	q := `SELECT id,campaign_id,generation,parent_id,attack_type,payload,mutations,bypassed_guardrail,success,response_snippet,created_at FROM feral_attacks WHERE 1=1`
	var args []any
	if campaignID != "" { q += " AND campaign_id=?"; args = append(args, campaignID) }
	if successOnly { q += " AND success=1" }
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Attack
	for rows.Next() {
		var a Attack
		if err := rows.Scan(&a.ID, &a.CampaignID, &a.Generation, &a.ParentID, &a.AttackType, &a.Payload, &a.Mutations, &a.BypassedGuardrail, &a.Success, &a.ResponseSnippet, &a.CreatedAt); err != nil {
			continue
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return out, nil
}

func (db *DB) ListVulnerabilities() ([]Vulnerability, error) {
	rows, err := db.conn.Query(`SELECT id,campaign_id,guardrail,attack_lineage,common_trait,severity,patched,discovered_at FROM feral_vulnerabilities ORDER BY discovered_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Vulnerability
	for rows.Next() {
		var v Vulnerability
		if err := rows.Scan(&v.ID, &v.CampaignID, &v.Guardrail, &v.AttackLineage, &v.CommonTrait, &v.Severity, &v.Patched, &v.DiscoveredAt); err != nil {
			continue
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var campaigns, attacks, successful, vulns int
	db.conn.QueryRow(`SELECT COUNT(*) FROM feral_campaigns`).Scan(&campaigns)
	db.conn.QueryRow(`SELECT COUNT(*) FROM feral_attacks`).Scan(&attacks)
	db.conn.QueryRow(`SELECT COUNT(*) FROM feral_attacks WHERE success=1`).Scan(&successful)
	db.conn.QueryRow(`SELECT COUNT(*) FROM feral_vulnerabilities`).Scan(&vulns)
	return map[string]any{"campaigns": campaigns, "total_attacks": attacks, "successful_attacks": successful, "vulnerabilities": vulns}, nil
}

func (db *DB) CreateAttack(a *Attack) error {
	_, err := db.conn.Exec(`INSERT INTO feral_attacks (id,campaign_id,generation,parent_id,attack_type,payload,mutations,bypassed_guardrail,success,response_snippet) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.CampaignID, a.Generation, a.ParentID, a.AttackType, a.Payload, a.Mutations, a.BypassedGuardrail, a.Success, a.ResponseSnippet)
	return err
}

func (db *DB) CreateVulnerability(v *Vulnerability) error {
	_, err := db.conn.Exec(`INSERT INTO feral_vulnerabilities (id,campaign_id,guardrail,attack_lineage,common_trait,severity) VALUES (?,?,?,?,?,?)`,
		v.ID, v.CampaignID, v.Guardrail, v.AttackLineage, v.CommonTrait, v.Severity)
	return err
}

func (db *DB) UpdateCampaign(id string, generation, totalAttacks, successfulAttacks int, status string) error {
	_, err := db.conn.Exec(`UPDATE feral_campaigns SET generation=?, total_attacks=?, successful_attacks=?, status=? WHERE id=?`,
		generation, totalAttacks, successfulAttacks, status, id)
	return err
}
