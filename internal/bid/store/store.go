// Package store provides SQLite persistence for auction history and provider quality.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
	"github.com/stockyard-dev/stockyard/internal/bid/quality"

	_ "modernc.org/sqlite"
)

// DB wraps a SQL database connection for bid persistence.
type DB struct {
	db *sql.DB
}

// Open creates a new DB backed by the SQLite file at path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS auctions (
	id               TEXT PRIMARY KEY,
	winner_provider  TEXT,
	winner_model     TEXT,
	bid_count        INTEGER,
	duration_ms      REAL,
	reason           TEXT,
	created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bids (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	auction_id     TEXT REFERENCES auctions(id),
	provider       TEXT,
	model          TEXT,
	price_per_1k   REAL,
	est_latency_ms REAL,
	quality_score  REAL,
	confidence     REAL
);

CREATE TABLE IF NOT EXISTS completed_requests (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	auction_id      TEXT,
	provider        TEXT,
	model           TEXT,
	bid_price       REAL,
	actual_latency_ms REAL,
	actual_tokens   INTEGER,
	actual_cost     REAL,
	success         BOOLEAN,
	error           TEXT,
	completed_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS provider_quality (
	provider        TEXT PRIMARY KEY,
	total_requests  INTEGER,
	successes       INTEGER,
	failures        INTEGER,
	avg_latency_ms  REAL,
	quality_bias    REAL,
	total_revenue   REAL,
	last_seen       DATETIME
);
`
	_, err := d.db.Exec(ddl)
	return err
}

// SaveAuction persists an auction result and all its bids.
func (d *DB) SaveAuction(result exchange.AuctionResult) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var winnerProvider, winnerModel string
	if result.Winner != nil {
		winnerProvider = result.Winner.ProviderID
		winnerModel = result.Winner.Model
	}

	reason := result.Reason
	if reason == "" {
		reason = result.NoBidsReason
	}

	_, err = tx.Exec(
		`INSERT INTO auctions (id, winner_provider, winner_model, bid_count, duration_ms, reason)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		result.RequestID, winnerProvider, winnerModel, result.BidCount,
		float64(result.AuctionTime.Milliseconds()), reason,
	)
	if err != nil {
		return fmt.Errorf("insert auction: %w", err)
	}

	for _, b := range result.AllBids {
		_, err = tx.Exec(
			`INSERT INTO bids (auction_id, provider, model, price_per_1k, est_latency_ms, quality_score, confidence)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			result.RequestID, b.ProviderID, b.Model, b.PricePer1K,
			float64(b.EstLatencyMs), b.QualityScore, b.ConfidenceScore,
		)
		if err != nil {
			return fmt.Errorf("insert bid: %w", err)
		}
	}

	return tx.Commit()
}

// ListAuctions returns the most recent auctions with their bids.
func (d *DB) ListAuctions(limit int) ([]exchange.AuctionResult, error) {
	rows, err := d.db.Query(
		`SELECT id, winner_provider, winner_model, bid_count, duration_ms, reason
		 FROM auctions ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []exchange.AuctionResult
	for rows.Next() {
		var (
			r               exchange.AuctionResult
			winnerProvider  string
			winnerModel     string
			durationMs      float64
			reason          string
		)
		if err := rows.Scan(&r.RequestID, &winnerProvider, &winnerModel, &r.BidCount, &durationMs, &reason); err != nil {
			return nil, err
		}
		r.AuctionTime = time.Duration(durationMs) * time.Millisecond
		r.Reason = reason
		if winnerProvider != "" {
			r.Winner = &exchange.Bid{ProviderID: winnerProvider, Model: winnerModel}
		}

		// Load bids for this auction
		bidRows, err := d.db.Query(
			`SELECT provider, model, price_per_1k, est_latency_ms, quality_score, confidence
			 FROM bids WHERE auction_id = ?`, r.RequestID)
		if err != nil {
			return nil, err
		}
		for bidRows.Next() {
			var b exchange.Bid
			var latMs float64
			if err := bidRows.Scan(&b.ProviderID, &b.Model, &b.PricePer1K, &latMs, &b.QualityScore, &b.ConfidenceScore); err != nil {
				bidRows.Close()
				return nil, err
			}
			b.EstLatencyMs = int(latMs)
			r.AllBids = append(r.AllBids, b)
		}
		bidRows.Close()

		results = append(results, r)
	}
	return results, rows.Err()
}

// SaveCompletedRequest persists a completed request record.
func (d *DB) SaveCompletedRequest(req exchange.CompletedRequest) error {
	_, err := d.db.Exec(
		`INSERT INTO completed_requests (auction_id, provider, model, bid_price, actual_latency_ms, actual_tokens, actual_cost, success, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.RequestID, req.ProviderID, req.Model, req.BidPrice,
		float64(req.ActualLatencyMs), req.ActualTokens, req.ActualCost,
		req.Success, req.Error,
	)
	return err
}

// GetProviderQuality retrieves the quality record for a single provider.
func (d *DB) GetProviderQuality(providerID string) (*quality.ProviderRecord, error) {
	row := d.db.QueryRow(
		`SELECT provider, total_requests, successes, failures, avg_latency_ms, quality_bias, total_revenue, last_seen
		 FROM provider_quality WHERE provider = ?`, providerID)

	var rec quality.ProviderRecord
	var lastSeen sql.NullTime
	err := row.Scan(&rec.ProviderID, &rec.TotalRequests, &rec.Successes, &rec.Failures,
		&rec.AvgLatencyMs, &rec.QualityBias, &rec.TotalRevenue, &lastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		rec.LastSeen = lastSeen.Time
	}
	rec.SuccessRate = 0
	if rec.TotalRequests > 0 {
		rec.SuccessRate = float64(rec.Successes) / float64(rec.TotalRequests)
	}
	return &rec, nil
}

// UpdateProviderQuality upserts the quality record for a provider.
func (d *DB) UpdateProviderQuality(providerID string, record *quality.ProviderRecord) error {
	_, err := d.db.Exec(
		`INSERT INTO provider_quality (provider, total_requests, successes, failures, avg_latency_ms, quality_bias, total_revenue, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(provider) DO UPDATE SET
			total_requests = excluded.total_requests,
			successes      = excluded.successes,
			failures       = excluded.failures,
			avg_latency_ms = excluded.avg_latency_ms,
			quality_bias   = excluded.quality_bias,
			total_revenue  = excluded.total_revenue,
			last_seen      = excluded.last_seen`,
		providerID, record.TotalRequests, record.Successes, record.Failures,
		record.AvgLatencyMs, record.QualityBias, record.TotalRevenue, record.LastSeen,
	)
	return err
}

// ListProviderQuality returns all provider quality records.
func (d *DB) ListProviderQuality() (map[string]*quality.ProviderRecord, error) {
	rows, err := d.db.Query(
		`SELECT provider, total_requests, successes, failures, avg_latency_ms, quality_bias, total_revenue, last_seen
		 FROM provider_quality`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make(map[string]*quality.ProviderRecord)
	for rows.Next() {
		var rec quality.ProviderRecord
		var lastSeen sql.NullTime
		if err := rows.Scan(&rec.ProviderID, &rec.TotalRequests, &rec.Successes, &rec.Failures,
			&rec.AvgLatencyMs, &rec.QualityBias, &rec.TotalRevenue, &lastSeen); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			rec.LastSeen = lastSeen.Time
		}
		if rec.TotalRequests > 0 {
			rec.SuccessRate = float64(rec.Successes) / float64(rec.TotalRequests)
		}
		records[rec.ProviderID] = &rec
	}
	return records, rows.Err()
}
