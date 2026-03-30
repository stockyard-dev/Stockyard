package storage

import (
	"log"
	"time"
)

// SpendRollup represents a daily spend summary.
type SpendRollup struct {
	Project        string  `json:"project"`
	Date           string  `json:"date"`
	TotalCost      float64 `json:"total_cost"`
	TotalRequests  int     `json:"total_requests"`
	TotalTokensIn  int     `json:"total_tokens_in"`
	TotalTokensOut int     `json:"total_tokens_out"`
}

// UpsertSpendRollup inserts or updates a daily spend rollup.
func (db *DB) UpsertSpendRollup(project string, cost float64, tokensIn, tokensOut int) error {
	date := time.Now().Format("2006-01-02")
	_, err := db.conn.Exec(`
		INSERT INTO spend_rollups (project, date, total_cost, total_requests, total_tokens_in, total_tokens_out)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(project, date) DO UPDATE SET
			total_cost = total_cost + excluded.total_cost,
			total_requests = total_requests + 1,
			total_tokens_in = total_tokens_in + excluded.total_tokens_in,
			total_tokens_out = total_tokens_out + excluded.total_tokens_out`,
		project, date, cost, tokensIn, tokensOut,
	)
	return err
}

// GetSpendHistory returns daily spend for a project over N days.
func (db *DB) GetSpendHistory(project string, days int) ([]SpendRollup, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := db.conn.Query(`
		SELECT project, date, total_cost, total_requests, total_tokens_in, total_tokens_out
		FROM spend_rollups
		WHERE (project = ? OR ? = '') AND date >= ?
		ORDER BY date DESC`,
		project, project, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rollups []SpendRollup
	for rows.Next() {
		var r SpendRollup
		if err := rows.Scan(&r.Project, &r.Date, &r.TotalCost, &r.TotalRequests,
			&r.TotalTokensIn, &r.TotalTokensOut); err != nil {
			return nil, err
		}
		rollups = append(rollups, r)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return rollups, nil
}

// GetTodaySpend returns today's total spend for a project.
func (db *DB) GetTodaySpend(project string) (float64, error) {
	date := time.Now().Format("2006-01-02")
	var total float64
	err := db.conn.QueryRow(
		"SELECT COALESCE(total_cost, 0) FROM spend_rollups WHERE project = ? AND date = ?",
		project, date,
	).Scan(&total)
	if err != nil {
		return 0, nil // No data yet
	}
	return total, nil
}

// GetMonthSpend returns this month's total spend for a project.
func (db *DB) GetMonthSpend(project string) (float64, error) {
	monthStart := time.Now().Format("2006-01") + "-01"
	var total float64
	err := db.conn.QueryRow(
		"SELECT COALESCE(SUM(total_cost), 0) FROM spend_rollups WHERE project = ? AND date >= ?",
		project, monthStart,
	).Scan(&total)
	if err != nil {
		return 0, nil
	}
	return total, nil
}

// --- Per-User Spend ---

// UserSpendRollup represents a daily spend summary for a user.
type UserSpendRollup struct {
	UserID         string  `json:"user_id"`
	Date           string  `json:"date"`
	TotalCost      float64 `json:"total_cost"`
	TotalRequests  int     `json:"total_requests"`
	TotalTokensIn  int     `json:"total_tokens_in"`
	TotalTokensOut int     `json:"total_tokens_out"`
}

// UpsertUserSpendRollup inserts or updates a daily user spend rollup.
func (db *DB) UpsertUserSpendRollup(userID string, cost float64, tokensIn, tokensOut int) error {
	date := time.Now().Format("2006-01-02")
	_, err := db.conn.Exec(`
		INSERT INTO user_spend_rollups (user_id, date, total_cost, total_requests, total_tokens_in, total_tokens_out)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(user_id, date) DO UPDATE SET
			total_cost = total_cost + excluded.total_cost,
			total_requests = total_requests + 1,
			total_tokens_in = total_tokens_in + excluded.total_tokens_in,
			total_tokens_out = total_tokens_out + excluded.total_tokens_out`,
		userID, date, cost, tokensIn, tokensOut,
	)
	return err
}

// GetUserSpendHistory returns daily spend for a user over N days.
func (db *DB) GetUserSpendHistory(userID string, days int) ([]UserSpendRollup, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := db.conn.Query(`
		SELECT user_id, date, total_cost, total_requests, total_tokens_in, total_tokens_out
		FROM user_spend_rollups
		WHERE user_id = ? AND date >= ?
		ORDER BY date DESC`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rollups []UserSpendRollup
	for rows.Next() {
		var r UserSpendRollup
		if err := rows.Scan(&r.UserID, &r.Date, &r.TotalCost, &r.TotalRequests,
			&r.TotalTokensIn, &r.TotalTokensOut); err != nil {
			return nil, err
		}
		rollups = append(rollups, r)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return rollups, nil
}

// --- User Spend Caps ---

// UserSpendCap represents per-user spending limits.
type UserSpendCap struct {
	UserID     string  `json:"user_id"`
	Tier       string  `json:"tier"`
	DailyCap   float64 `json:"daily_cap"`
	MonthlyCap float64 `json:"monthly_cap"`
	SoftCap    bool    `json:"soft_cap"`
}

// GetUserSpendCap returns the spend cap for a user, or nil if none set.
func (db *DB) GetUserSpendCap(userID string) (*UserSpendCap, error) {
	var cap UserSpendCap
	var softCap int
	err := db.conn.QueryRow(`
		SELECT user_id, tier, daily_cap, monthly_cap, soft_cap
		FROM user_spend_caps WHERE user_id = ?`, userID).
		Scan(&cap.UserID, &cap.Tier, &cap.DailyCap, &cap.MonthlyCap, &softCap)
	if err != nil {
		return nil, nil // No cap configured
	}
	cap.SoftCap = softCap != 0
	return &cap, nil
}

// SetUserSpendCap creates or updates a user's spend cap.
func (db *DB) SetUserSpendCap(userID string, dailyCap, monthlyCap float64, softCap bool) error {
	soft := 0
	if softCap {
		soft = 1
	}
	_, err := db.conn.Exec(`
		INSERT INTO user_spend_caps (user_id, daily_cap, monthly_cap, soft_cap, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(user_id) DO UPDATE SET
			daily_cap = excluded.daily_cap,
			monthly_cap = excluded.monthly_cap,
			soft_cap = excluded.soft_cap,
			updated_at = datetime('now')`,
		userID, dailyCap, monthlyCap, soft,
	)
	return err
}

// --- Usage Metering ---

// UpsertUsageMetering inserts or updates a usage metering record.
func (db *DB) UpsertUsageMetering(dimension, dimensionKey string, cost float64, tokensIn, tokensOut int) error {
	date := time.Now().Format("2006-01-02")
	_, err := db.conn.Exec(`
		INSERT INTO usage_metering (dimension, dimension_key, date, requests, tokens_in, tokens_out, cost_usd)
		VALUES (?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(dimension, dimension_key, date) DO UPDATE SET
			requests = requests + 1,
			tokens_in = tokens_in + excluded.tokens_in,
			tokens_out = tokens_out + excluded.tokens_out,
			cost_usd = cost_usd + excluded.cost_usd`,
		dimension, dimensionKey, date, tokensIn, tokensOut, cost,
	)
	return err
}

// --- Team Spend ---

// UpsertTeamSpendRollup inserts or updates a daily team spend rollup.
func (db *DB) UpsertTeamSpendRollup(teamID string, cost float64, tokensIn, tokensOut int) error {
	if teamID == "" {
		return nil
	}
	date := time.Now().Format("2006-01-02")
	_, err := db.conn.Exec(`
		INSERT INTO team_spend_rollups (team_id, date, total_cost, total_requests, total_tokens_in, total_tokens_out)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(team_id, date) DO UPDATE SET
			total_cost = total_cost + excluded.total_cost,
			total_requests = total_requests + 1,
			total_tokens_in = total_tokens_in + excluded.total_tokens_in,
			total_tokens_out = total_tokens_out + excluded.total_tokens_out`,
		teamID, date, cost, tokensIn, tokensOut,
	)
	return err
}
