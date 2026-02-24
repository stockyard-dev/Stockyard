package storage

import (
	"database/sql"
	"time"
)

// RequestLog represents a logged request in the database.
type RequestLog struct {
	ID             string        `json:"id"`
	Timestamp      time.Time     `json:"timestamp"`
	Project        string        `json:"project"`
	UserID         string        `json:"user_id,omitempty"`
	Provider       string        `json:"provider"`
	Model          string        `json:"model"`
	TokensIn       int           `json:"tokens_in"`
	TokensOut      int           `json:"tokens_out"`
	CostUSD        float64       `json:"cost_usd"`
	LatencyMs      int64         `json:"latency_ms"`
	Status         int           `json:"status"`
	CacheHit       bool          `json:"cache_hit"`
	ValidationPass *bool         `json:"validation_pass,omitempty"`
	FailoverUsed   bool          `json:"failover_used"`
	RequestBody    string        `json:"request_body,omitempty"`
	ResponseBody   string        `json:"response_body,omitempty"`
	Error          string        `json:"error,omitempty"`
}

// InsertRequest logs a proxied request to the database.
func (db *DB) InsertRequest(r *RequestLog) error {
	_, err := db.conn.Exec(`
		INSERT INTO requests (id, timestamp, project, user_id, provider, model,
			tokens_in, tokens_out, cost_usd, latency_ms, status, cache_hit,
			validation_pass, failover_used, request_body, response_body, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Timestamp.Format(time.RFC3339), r.Project, r.UserID,
		r.Provider, r.Model, r.TokensIn, r.TokensOut, r.CostUSD,
		r.LatencyMs, r.Status, r.CacheHit, r.ValidationPass,
		r.FailoverUsed, r.RequestBody, r.ResponseBody, r.Error,
	)
	return err
}

// ListRequests returns paginated request logs.
func (db *DB) ListRequests(project string, limit, offset int) ([]RequestLog, int, error) {
	var total int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM requests WHERE project = ? OR ? = ''",
		project, project,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.conn.Query(`
		SELECT id, timestamp, project, COALESCE(user_id, ''), provider, model,
			tokens_in, tokens_out, cost_usd, latency_ms, status, cache_hit,
			failover_used, COALESCE(error, '')
		FROM requests
		WHERE project = ? OR ? = ''
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?`,
		project, project, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []RequestLog
	for rows.Next() {
		var r RequestLog
		var ts string
		if err := rows.Scan(&r.ID, &ts, &r.Project, &r.UserID, &r.Provider,
			&r.Model, &r.TokensIn, &r.TokensOut, &r.CostUSD, &r.LatencyMs,
			&r.Status, &r.CacheHit, &r.FailoverUsed, &r.Error); err != nil {
			return nil, 0, err
		}
		r.Timestamp, _ = time.Parse(time.RFC3339, ts)
		logs = append(logs, r)
	}

	return logs, total, nil
}

// GetRequest returns a single request with full bodies.
func (db *DB) GetRequest(id string) (*RequestLog, error) {
	var r RequestLog
	var ts string
	err := db.conn.QueryRow(`
		SELECT id, timestamp, project, COALESCE(user_id, ''), provider, model,
			tokens_in, tokens_out, cost_usd, latency_ms, status, cache_hit,
			failover_used, COALESCE(request_body, ''), COALESCE(response_body, ''),
			COALESCE(error, '')
		FROM requests WHERE id = ?`, id,
	).Scan(&r.ID, &ts, &r.Project, &r.UserID, &r.Provider, &r.Model,
		&r.TokensIn, &r.TokensOut, &r.CostUSD, &r.LatencyMs, &r.Status,
		&r.CacheHit, &r.FailoverUsed, &r.RequestBody, &r.ResponseBody, &r.Error)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Timestamp, _ = time.Parse(time.RFC3339, ts)
	return &r, nil
}
