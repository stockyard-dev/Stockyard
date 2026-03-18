package engine

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/features"
)

// RegisterGuardrailRoutes mounts guardrail management routes.
func RegisterGuardrailRoutes(mux *http.ServeMux, mgr *features.GuardrailManager, conn *sql.DB) {
	// GET /api/guardrails — list all rules
	mux.HandleFunc("GET /api/guardrails", func(w http.ResponseWriter, r *http.Request) {
		project := r.URL.Query().Get("project")
		if project == "" {
			project = "*"
		}
		rules := mgr.GetRules(project)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"rules": rules})
	})

	// POST /api/guardrails — create a rule
	mux.HandleFunc("POST /api/guardrails", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Pattern     string `json:"pattern"`
			Action      string `json:"action"`
			Project     string `json:"project"`
			Priority    int    `json:"priority"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if body.Name == "" || body.Pattern == "" {
			jsonError(w, http.StatusBadRequest, "name and pattern are required")
			return
		}
		if body.Type == "" {
			body.Type = "custom"
		}
		if body.Action == "" {
			body.Action = "log"
		}
		if body.Project == "" {
			body.Project = "*"
		}

		res, err := conn.Exec(`
			INSERT INTO guardrail_rules (name, type, pattern, action, project, priority, description)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			body.Name, body.Type, body.Pattern, body.Action, body.Project, body.Priority, body.Description)
		if err != nil {
			log.Printf("[guardrails] create error: %v", err)
			jsonError(w, http.StatusInternalServerError, "failed to create rule")
			return
		}
		id, _ := res.LastInsertId()
		mgr.Invalidate()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "created"})
	})

	// GET /api/guardrails/{id} — get a single rule
	mux.HandleFunc("GET /api/guardrails/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var rule features.GuardrailRule
		var enabled int
		err := conn.QueryRow(`
			SELECT id, name, type, pattern, action, project, enabled, priority, description
			FROM guardrail_rules WHERE id = ?`, id).
			Scan(&rule.ID, &rule.Name, &rule.Type, &rule.Pattern, &rule.Action,
				&rule.Project, &enabled, &rule.Priority, &rule.Description)
		if err != nil {
			jsonError(w, http.StatusNotFound, "rule not found")
			return
		}
		rule.Enabled = enabled == 1
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	})

	// PUT /api/guardrails/{id} — update a rule
	mux.HandleFunc("PUT /api/guardrails/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Name        *string `json:"name"`
			Type        *string `json:"type"`
			Pattern     *string `json:"pattern"`
			Action      *string `json:"action"`
			Project     *string `json:"project"`
			Enabled     *bool   `json:"enabled"`
			Priority    *int    `json:"priority"`
			Description *string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// Build dynamic UPDATE
		updates := ""
		var args []any
		addField := func(col, val string) {
			if updates != "" {
				updates += ", "
			}
			updates += col + " = ?"
			args = append(args, val)
		}
		if body.Name != nil {
			addField("name", *body.Name)
		}
		if body.Type != nil {
			addField("type", *body.Type)
		}
		if body.Pattern != nil {
			addField("pattern", *body.Pattern)
		}
		if body.Action != nil {
			addField("action", *body.Action)
		}
		if body.Project != nil {
			addField("project", *body.Project)
		}
		if body.Description != nil {
			addField("description", *body.Description)
		}
		if body.Enabled != nil {
			enabled := "0"
			if *body.Enabled {
				enabled = "1"
			}
			addField("enabled", enabled)
		}
		if body.Priority != nil {
			if updates != "" {
				updates += ", "
			}
			updates += "priority = ?"
			args = append(args, *body.Priority)
		}

		if updates == "" {
			jsonError(w, http.StatusBadRequest, "no fields to update")
			return
		}
		updates += ", updated_at = datetime('now')"
		args = append(args, id)

		_, err := conn.Exec("UPDATE guardrail_rules SET "+updates+" WHERE id = ?", args...)
		if err != nil {
			log.Printf("[guardrails] update error: %v", err)
			jsonError(w, http.StatusInternalServerError, "failed to update rule")
			return
		}
		mgr.Invalidate()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	// DELETE /api/guardrails/{id} — delete a rule
	mux.HandleFunc("DELETE /api/guardrails/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		_, err := conn.Exec("DELETE FROM guardrail_rules WHERE id = ?", id)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to delete rule")
			return
		}
		mgr.Invalidate()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	})

	// POST /api/guardrails/test — dry-run text through rules
	mux.HandleFunc("POST /api/guardrails/test", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text    string `json:"text"`
			Project string `json:"project"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if body.Text == "" {
			jsonError(w, http.StatusBadRequest, "text is required")
			return
		}
		if body.Project == "" {
			body.Project = "*"
		}

		matches := mgr.ScanText(body.Text, body.Project, "test")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"matches": matches,
			"count":   len(matches),
		})
	})

	// GET /api/guardrails/stats — analytics counters
	mux.HandleFunc("GET /api/guardrails/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mgr.Stats())
	})

	// GET /api/guardrails/events — recent violation events
	mux.HandleFunc("GET /api/guardrails/events", func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}

		rows, err := conn.Query(`
			SELECT id, timestamp, rule_name, rule_type, action, direction, project, model, match_text, request_id
			FROM guardrail_events ORDER BY timestamp DESC LIMIT ?`, limit)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to query events")
			return
		}
		defer rows.Close()

		var events []map[string]any
		for rows.Next() {
			var id int64
			var ts, ruleName, ruleType, action, direction, project, model string
			var matchText, requestID sql.NullString
			if err := rows.Scan(&id, &ts, &ruleName, &ruleType, &action, &direction, &project, &model, &matchText, &requestID); err != nil {
				continue
			}
			e := map[string]any{
				"id": id, "timestamp": ts, "rule_name": ruleName, "rule_type": ruleType,
				"action": action, "direction": direction, "project": project, "model": model,
			}
			if matchText.Valid {
				e["match_text"] = matchText.String
			}
			if requestID.Valid {
				e["request_id"] = requestID.String
			}
			events = append(events, e)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"events": events})
	})
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
