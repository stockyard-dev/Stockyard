package apiserver

// Admin endpoints for the Apr 12 2026 trial_drip duplicate-send incident.
// Lets an operator list suspect rows (where dayN_sent=0 but the email was
// likely already delivered per Resend logs) and flip individual flags to 1
// after cross-referencing. Scoped narrowly — one row + one flag per call.

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type trialDripSuspect struct {
	ID         int    `json:"id"`
	Email      string `json:"email"`
	BundleSlug string `json:"bundle_slug"`
	TrialStart string `json:"trial_start"`
	Day3Sent   int    `json:"day3_sent"`
	Day7Sent   int    `json:"day7_sent"`
	Day12Sent  int    `json:"day12_sent"`
	Day14Sent  int    `json:"day14_sent"`
}

// GET /api/admin/trial-drip/suspects
// Returns rows where any dayN_sent=0 but the trial is past that day — i.e.
// candidates for the "already delivered but flag never flipped" pattern.
func (s *Server) handleAdminTrialDripSuspects(w http.ResponseWriter, r *http.Request) {
	if s.webhook == nil || s.webhook.trialDrip == nil {
		http.Error(w, "trial_drip runner not configured", http.StatusServiceUnavailable)
		return
	}
	rows, err := s.webhook.trialDrip.db.Query(`
		SELECT id, email, bundle_slug, trial_start,
		       day3_sent, day7_sent, day12_sent, day14_sent
		FROM trial_drip
		WHERE converted = 0 AND cancelled = 0
		  AND (
		    (day3_sent  = 0 AND datetime(trial_start) < datetime('now', '-3 days'))  OR
		    (day7_sent  = 0 AND datetime(trial_start) < datetime('now', '-7 days'))  OR
		    (day12_sent = 0 AND datetime(trial_start) < datetime('now', '-12 days')) OR
		    (day14_sent = 0 AND datetime(trial_start) < datetime('now', '-14 days'))
		  )
		ORDER BY trial_start`)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := []trialDripSuspect{}
	for rows.Next() {
		var s trialDripSuspect
		if err := rows.Scan(&s.ID, &s.Email, &s.BundleSlug, &s.TrialStart,
			&s.Day3Sent, &s.Day7Sent, &s.Day12Sent, &s.Day14Sent); err != nil {
			http.Error(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"count": len(out), "suspects": out})
}

// POST /api/admin/trial-drip/mark-sent
// Body: {"id": 123, "flag": "day3_sent"}
// Flips one flag on one row to 1. Intended for post-incident cleanup after
// cross-referencing Resend delivery logs. Returns {ok, rows_affected}.
func (s *Server) handleAdminTrialDripMarkSent(w http.ResponseWriter, r *http.Request) {
	if s.webhook == nil || s.webhook.trialDrip == nil {
		http.Error(w, "trial_drip runner not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		ID   int    `json:"id"`
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.ID <= 0 {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if !trialDripColumns[body.Flag] {
		http.Error(w, "flag must be day3_sent|day7_sent|day12_sent|day14_sent", http.StatusBadRequest)
		return
	}
	q := fmt.Sprintf(`UPDATE trial_drip SET %s = 1 WHERE id = ?`, body.Flag)
	res, err := s.webhook.trialDrip.db.Exec(q, body.ID)
	if err != nil {
		http.Error(w, "update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "rows_affected": n, "id": body.ID, "flag": body.Flag})
}
