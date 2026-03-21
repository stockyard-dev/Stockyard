package studio

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const distillSchema = `
CREATE TABLE IF NOT EXISTS distillation_jobs (
    id TEXT PRIMARY KEY,
    model TEXT NOT NULL,
    date_from TEXT NOT NULL,
    date_to TEXT NOT NULL,
    sample_count INTEGER DEFAULT 0,
    output_format TEXT DEFAULT 'jsonl',
    status TEXT DEFAULT 'pending',
    result TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);
`

func migrateDistillSchema(conn *sql.DB) {
	conn.Exec(distillSchema)
}

func registerDistillRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("POST /api/studio/distill", handleDistill(conn))
	mux.HandleFunc("GET /api/studio/distill/{id}", handleDistillStatus(conn))
	log.Printf("[studio] distillation routes registered")
}

func handleDistill(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			DateFrom string `json:"date_from"`
			DateTo   string `json:"date_to"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "model required"})
			return
		}
		if req.DateFrom == "" {
			req.DateFrom = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		}
		if req.DateTo == "" {
			req.DateTo = time.Now().Format("2006-01-02")
		}

		id := generateTestID("dst_")
		now := time.Now().UTC().Format(time.RFC3339)
		conn.Exec(`INSERT INTO distillation_jobs (id, model, date_from, date_to, status, created_at) VALUES (?, ?, ?, ?, 'running', ?)`,
			id, req.Model, req.DateFrom, req.DateTo, now)

		// Extract training data in background.
		go func() {
			rows, err := conn.Query(`
				SELECT model, metadata_json
				FROM observe_traces
				WHERE model = ? AND created_at >= ? AND created_at <= ? AND status = 'ok'
				ORDER BY created_at
				LIMIT 10000
			`, req.Model, req.DateFrom, req.DateTo)
			if err != nil {
				conn.Exec("UPDATE distillation_jobs SET status = 'failed', result = ? WHERE id = ?",
					fmt.Sprintf(`{"error":"%s"}`, err.Error()), id)
				return
			}
			defer rows.Close()

			var samples int
			var trainingData []map[string]any
			for rows.Next() {
				var model, metaJSON string
				rows.Scan(&model, &metaJSON)
				var meta map[string]any
				json.Unmarshal([]byte(metaJSON), &meta)

				// Build JSONL training format.
				trainingData = append(trainingData, map[string]any{
					"model": model,
					"metadata": meta,
				})
				samples++
			}

			resultJSON, _ := json.Marshal(map[string]any{
				"samples":       samples,
				"format":        "jsonl",
				"training_data": trainingData,
			})
			completedAt := time.Now().UTC().Format(time.RFC3339)
			conn.Exec("UPDATE distillation_jobs SET status = 'completed', sample_count = ?, result = ?, completed_at = ? WHERE id = ?",
				samples, string(resultJSON), completedAt, id)
			log.Printf("[distill] job %s complete: %d samples from %s", id, samples, req.Model)
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]any{
			"id": id, "model": req.Model, "status": "running",
			"date_from": req.DateFrom, "date_to": req.DateTo,
		})
	}
}

func handleDistillStatus(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var model, dateFrom, dateTo, status, resultJSON, createdAt string
		var completedAt sql.NullString
		var sampleCount int

		err := conn.QueryRow(`SELECT model, date_from, date_to, sample_count, status, result, created_at, completed_at
			FROM distillation_jobs WHERE id = ?`, id).
			Scan(&model, &dateFrom, &dateTo, &sampleCount, &status, &resultJSON, &createdAt, &completedAt)
		if err != nil {
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "job not found"})
			return
		}

		var result any
		json.Unmarshal([]byte(resultJSON), &result)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": id, "model": model, "date_from": dateFrom, "date_to": dateTo,
			"sample_count": sampleCount, "status": status, "result": result,
			"created_at": createdAt, "completed_at": completedAt.String,
		})
	}
}
