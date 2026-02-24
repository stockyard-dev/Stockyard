package storage

import "time"

// SpendRollup represents a daily spend summary.
type SpendRollup struct {
	Project       string  `json:"project"`
	Date          string  `json:"date"`
	TotalCost     float64 `json:"total_cost"`
	TotalRequests int     `json:"total_requests"`
	TotalTokensIn int     `json:"total_tokens_in"`
	TotalTokensOut int    `json:"total_tokens_out"`
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
