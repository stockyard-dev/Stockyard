// Package engine — replaymode provides request replay: re-send any logged trace
// to a different model/provider and compare cost, latency, and output.
package engine

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// MigrateReplayMode creates the replay_runs table and adds request_body column
// to observe_traces if missing.
func MigrateReplayMode(conn *sql.DB) {
	conn.Exec(`ALTER TABLE observe_traces ADD COLUMN request_body TEXT DEFAULT ''`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS replay_runs (
		id TEXT PRIMARY KEY,
		source_trace_id TEXT NOT NULL,
		source_model TEXT NOT NULL,
		source_provider TEXT NOT NULL,
		target_model TEXT NOT NULL,
		target_provider TEXT NOT NULL,
		request_body TEXT NOT NULL,
		source_response TEXT NOT NULL DEFAULT '',
		target_response TEXT NOT NULL DEFAULT '',
		source_tokens_in INTEGER DEFAULT 0,
		source_tokens_out INTEGER DEFAULT 0,
		source_cost_usd REAL DEFAULT 0,
		source_duration_ms INTEGER DEFAULT 0,
		target_tokens_in INTEGER DEFAULT 0,
		target_tokens_out INTEGER DEFAULT 0,
		target_cost_usd REAL DEFAULT 0,
		target_duration_ms INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		error_message TEXT DEFAULT '',
		created_at TEXT DEFAULT (datetime('now'))
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_replay_source ON replay_runs(source_trace_id)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_replay_created ON replay_runs(created_at)`)
}

// RegisterReplayRoutes registers all replay mode API routes.
func RegisterReplayRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("POST /api/replay", handleReplay(conn))
	mux.HandleFunc("GET /api/replay/runs", handleReplayRuns(conn))
	mux.HandleFunc("GET /api/replay/runs/{id}", handleReplayRun(conn))
	mux.HandleFunc("GET /api/replay/compare/{id}", handleReplayCompare(conn))
	mux.HandleFunc("GET /api/replay/stats", handleReplayStats(conn))
	mux.HandleFunc("POST /api/replay/batch", handleReplayBatch(conn))
}

// handleReplay replays a trace against a target model.
// POST /api/replay
// Body: {"trace_id": "...", "target_model": "claude-3-5-sonnet-20241022", "target_provider": "anthropic"}
func handleReplay(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TraceID        string   `json:"trace_id"`
			TargetModel    string   `json:"target_model"`
			TargetProvider string   `json:"target_provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			replayJSON(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.TraceID == "" {
			replayJSON(w, 400, map[string]string{"error": "trace_id required"})
			return
		}
		if req.TargetModel == "" {
			replayJSON(w, 400, map[string]string{"error": "target_model required"})
			return
		}

		// Auto-detect provider from model if not specified
		if req.TargetProvider == "" {
			req.TargetProvider = provider.ProviderForModel(req.TargetModel)
			if req.TargetProvider == "" {
				replayJSON(w, 400, map[string]string{"error": "could not detect provider for model — set target_provider"})
				return
			}
		}

		// Load original trace
		var srcModel, srcProvider, srcResponseBody, requestBody string
		var srcTokensIn, srcTokensOut, srcDurationMs int64
		var srcCostUSD float64
		err := conn.QueryRow(`SELECT model, provider, COALESCE(response_body,''), COALESCE(request_body,''),
			COALESCE(tokens_in,0), COALESCE(tokens_out,0), COALESCE(cost_usd,0), COALESCE(duration_ms,0)
			FROM observe_traces WHERE id = ?`, req.TraceID).Scan(
			&srcModel, &srcProvider, &srcResponseBody, &requestBody,
			&srcTokensIn, &srcTokensOut, &srcCostUSD, &srcDurationMs)
		if err != nil {
			replayJSON(w, 404, map[string]string{"error": "trace not found"})
			return
		}
		if requestBody == "" {
			replayJSON(w, 422, map[string]string{"error": "trace has no stored request body — only traces recorded after replay mode was enabled can be replayed"})
			return
		}

		// Parse the stored request body
		var parsedReq struct {
			Model       string             `json:"model"`
			Messages    []provider.Message  `json:"messages"`
			Temperature *float64            `json:"temperature,omitempty"`
			MaxTokens   *int                `json:"max_tokens,omitempty"`
		}
		if err := json.Unmarshal([]byte(requestBody), &parsedReq); err != nil {
			replayJSON(w, 422, map[string]string{"error": "cannot parse stored request body: " + err.Error()})
			return
		}
		if len(parsedReq.Messages) == 0 {
			replayJSON(w, 422, map[string]string{"error": "stored request has no messages"})
			return
		}

		// Find the target provider
		p, ok := GlobalProviders[req.TargetProvider]
		if !ok {
			available := make([]string, 0, len(GlobalProviders))
			for k := range GlobalProviders {
				available = append(available, k)
			}
			replayJSON(w, 422, map[string]any{
				"error":               "provider not configured: " + req.TargetProvider,
				"available_providers": available,
			})
			return
		}

		// Build the replay request
		provReq := &provider.Request{
			Model:       req.TargetModel,
			Messages:    parsedReq.Messages,
			Temperature: parsedReq.Temperature,
			MaxTokens:   parsedReq.MaxTokens,
			Extra:       map[string]any{},
		}

		// Execute the replay
		runID := genReplayID()
		start := time.Now()
		resp, sendErr := p.Send(r.Context(), provReq)
		duration := time.Since(start)

		status := "ok"
		var targetResponse string
		var targetTokensIn, targetTokensOut int64
		var targetCostUSD float64
		errMsg := ""

		if sendErr != nil {
			status = "error"
			errMsg = sendErr.Error()
		} else if resp != nil {
			if len(resp.Choices) > 0 {
				targetResponse = resp.Choices[0].Message.Content
				if len(targetResponse) > 10240 {
					targetResponse = targetResponse[:10240]
				}
			}
			targetTokensIn = int64(resp.Usage.PromptTokens)
			targetTokensOut = int64(resp.Usage.CompletionTokens)
			targetCostUSD = float64(targetTokensIn)/1000*0.002 + float64(targetTokensOut)/1000*0.006
		}

		// Store the run
		conn.Exec(`INSERT INTO replay_runs
			(id, source_trace_id, source_model, source_provider, target_model, target_provider,
			 request_body, source_response, target_response,
			 source_tokens_in, source_tokens_out, source_cost_usd, source_duration_ms,
			 target_tokens_in, target_tokens_out, target_cost_usd, target_duration_ms,
			 status, error_message, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			runID, req.TraceID, srcModel, srcProvider, req.TargetModel, req.TargetProvider,
			requestBody, srcResponseBody, targetResponse,
			srcTokensIn, srcTokensOut, srcCostUSD, srcDurationMs,
			targetTokensIn, targetTokensOut, targetCostUSD, duration.Milliseconds(),
			status, errMsg, time.Now().UTC().Format(time.RFC3339))

		// Build comparison result
		result := map[string]any{
			"id":     runID,
			"status": status,
			"source": map[string]any{
				"trace_id":   req.TraceID,
				"model":      srcModel,
				"provider":   srcProvider,
				"tokens_in":  srcTokensIn,
				"tokens_out": srcTokensOut,
				"cost_usd":   srcCostUSD,
				"latency_ms": srcDurationMs,
				"response":   truncate(srcResponseBody, 500),
			},
			"target": map[string]any{
				"model":      req.TargetModel,
				"provider":   req.TargetProvider,
				"tokens_in":  targetTokensIn,
				"tokens_out": targetTokensOut,
				"cost_usd":   targetCostUSD,
				"latency_ms": duration.Milliseconds(),
				"response":   truncate(targetResponse, 500),
			},
		}

		if sendErr != nil {
			result["error"] = errMsg
		} else {
			// Add deltas
			costDelta := targetCostUSD - srcCostUSD
			latencyDelta := duration.Milliseconds() - srcDurationMs
			tokenDelta := (targetTokensIn + targetTokensOut) - (srcTokensIn + srcTokensOut)
			result["delta"] = map[string]any{
				"cost_usd":    costDelta,
				"latency_ms":  latencyDelta,
				"total_tokens": tokenDelta,
			}
		}

		replayJSON(w, 200, result)
	}
}

// handleReplayBatch replays a trace against multiple models at once.
// POST /api/replay/batch
// Body: {"trace_id": "...", "targets": [{"model": "...", "provider": "..."}]}
func handleReplayBatch(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			TraceID string `json:"trace_id"`
			Targets []struct {
				Model    string `json:"model"`
				Provider string `json:"provider"`
			} `json:"targets"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			replayJSON(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.TraceID == "" || len(req.Targets) == 0 {
			replayJSON(w, 400, map[string]string{"error": "trace_id and targets required"})
			return
		}
		if len(req.Targets) > 5 {
			replayJSON(w, 400, map[string]string{"error": "max 5 targets per batch"})
			return
		}

		// Load original trace
		var srcModel, srcProvider, srcResponseBody, requestBody string
		var srcTokensIn, srcTokensOut, srcDurationMs int64
		var srcCostUSD float64
		err := conn.QueryRow(`SELECT model, provider, COALESCE(response_body,''), COALESCE(request_body,''),
			COALESCE(tokens_in,0), COALESCE(tokens_out,0), COALESCE(cost_usd,0), COALESCE(duration_ms,0)
			FROM observe_traces WHERE id = ?`, req.TraceID).Scan(
			&srcModel, &srcProvider, &srcResponseBody, &requestBody,
			&srcTokensIn, &srcTokensOut, &srcCostUSD, &srcDurationMs)
		if err != nil {
			replayJSON(w, 404, map[string]string{"error": "trace not found"})
			return
		}
		if requestBody == "" {
			replayJSON(w, 422, map[string]string{"error": "trace has no stored request body"})
			return
		}

		var parsedReq struct {
			Model       string             `json:"model"`
			Messages    []provider.Message  `json:"messages"`
			Temperature *float64            `json:"temperature,omitempty"`
			MaxTokens   *int                `json:"max_tokens,omitempty"`
		}
		if err := json.Unmarshal([]byte(requestBody), &parsedReq); err != nil {
			replayJSON(w, 422, map[string]string{"error": "cannot parse stored request body"})
			return
		}

		source := map[string]any{
			"trace_id":   req.TraceID,
			"model":      srcModel,
			"provider":   srcProvider,
			"tokens_in":  srcTokensIn,
			"tokens_out": srcTokensOut,
			"cost_usd":   srcCostUSD,
			"latency_ms": srcDurationMs,
			"response":   truncate(srcResponseBody, 300),
		}

		type batchResult struct {
			RunID      string `json:"run_id"`
			Model      string `json:"model"`
			Provider   string `json:"provider"`
			Status     string `json:"status"`
			Response   string `json:"response,omitempty"`
			TokensIn   int64  `json:"tokens_in"`
			TokensOut  int64  `json:"tokens_out"`
			CostUSD    float64 `json:"cost_usd"`
			LatencyMs  int64  `json:"latency_ms"`
			CostDelta  float64 `json:"cost_delta"`
			SpeedDelta int64  `json:"speed_delta"`
			Error      string `json:"error,omitempty"`
		}

		results := make([]batchResult, 0, len(req.Targets))

		for _, target := range req.Targets {
			targetProvider := target.Provider
			if targetProvider == "" {
				targetProvider = provider.ProviderForModel(target.Model)
			}
			p, ok := GlobalProviders[targetProvider]
			if !ok {
				results = append(results, batchResult{
					Model: target.Model, Provider: targetProvider,
					Status: "error", Error: "provider not configured",
				})
				continue
			}

			provReq := &provider.Request{
				Model:       target.Model,
				Messages:    parsedReq.Messages,
				Temperature: parsedReq.Temperature,
				MaxTokens:   parsedReq.MaxTokens,
				Extra:       map[string]any{},
			}

			runID := genReplayID()
			start := time.Now()
			resp, sendErr := p.Send(r.Context(), provReq)
			dur := time.Since(start)

			br := batchResult{RunID: runID, Model: target.Model, Provider: targetProvider}
			if sendErr != nil {
				br.Status = "error"
				br.Error = sendErr.Error()
			} else if resp != nil {
				br.Status = "ok"
				if len(resp.Choices) > 0 {
					br.Response = truncate(resp.Choices[0].Message.Content, 300)
				}
				br.TokensIn = int64(resp.Usage.PromptTokens)
				br.TokensOut = int64(resp.Usage.CompletionTokens)
				br.CostUSD = float64(br.TokensIn)/1000*0.002 + float64(br.TokensOut)/1000*0.006
				br.LatencyMs = dur.Milliseconds()
				br.CostDelta = br.CostUSD - srcCostUSD
				br.SpeedDelta = br.LatencyMs - srcDurationMs
			}

			// Store run
			targetResp := ""
			if resp != nil && len(resp.Choices) > 0 {
				targetResp = resp.Choices[0].Message.Content
				if len(targetResp) > 10240 {
					targetResp = targetResp[:10240]
				}
			}
			conn.Exec(`INSERT INTO replay_runs
				(id, source_trace_id, source_model, source_provider, target_model, target_provider,
				 request_body, source_response, target_response,
				 source_tokens_in, source_tokens_out, source_cost_usd, source_duration_ms,
				 target_tokens_in, target_tokens_out, target_cost_usd, target_duration_ms,
				 status, error_message, created_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				runID, req.TraceID, srcModel, srcProvider, target.Model, targetProvider,
				requestBody, srcResponseBody, targetResp,
				srcTokensIn, srcTokensOut, srcCostUSD, srcDurationMs,
				br.TokensIn, br.TokensOut, br.CostUSD, br.LatencyMs,
				br.Status, br.Error, time.Now().UTC().Format(time.RFC3339))

			results = append(results, br)
		}

		replayJSON(w, 200, map[string]any{
			"source":  source,
			"results": results,
			"count":   len(results),
		})
	}
}

// handleReplayRuns lists recent replay runs.
// GET /api/replay/runs?limit=20&model=gpt-4o
func handleReplayRuns(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		modelFilter := r.URL.Query().Get("model")

		query := `SELECT id, source_trace_id, source_model, source_provider, target_model, target_provider,
			source_tokens_in, source_tokens_out, source_cost_usd, source_duration_ms,
			target_tokens_in, target_tokens_out, target_cost_usd, target_duration_ms,
			status, COALESCE(error_message,''), created_at FROM replay_runs`
		args := []any{}

		if modelFilter != "" {
			query += ` WHERE target_model = ? OR source_model = ?`
			args = append(args, modelFilter, modelFilter)
		}
		query += ` ORDER BY created_at DESC LIMIT ?`
		args = append(args, limit)

		rows, err := conn.Query(query, args...)
		if err != nil {
			replayJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		type runSummary struct {
			ID             string  `json:"id"`
			SourceTraceID  string  `json:"source_trace_id"`
			SourceModel    string  `json:"source_model"`
			SourceProvider string  `json:"source_provider"`
			TargetModel    string  `json:"target_model"`
			TargetProvider string  `json:"target_provider"`
			SrcTokensIn    int64   `json:"source_tokens_in"`
			SrcTokensOut   int64   `json:"source_tokens_out"`
			SrcCostUSD     float64 `json:"source_cost_usd"`
			SrcLatencyMs   int64   `json:"source_latency_ms"`
			TgtTokensIn    int64   `json:"target_tokens_in"`
			TgtTokensOut   int64   `json:"target_tokens_out"`
			TgtCostUSD     float64 `json:"target_cost_usd"`
			TgtLatencyMs   int64   `json:"target_latency_ms"`
			CostDelta      float64 `json:"cost_delta"`
			SpeedDelta     int64   `json:"speed_delta"`
			Status         string  `json:"status"`
			Error          string  `json:"error,omitempty"`
			CreatedAt      string  `json:"created_at"`
		}

		runs := make([]runSummary, 0)
		for rows.Next() {
			var rs runSummary
			if err := rows.Scan(&rs.ID, &rs.SourceTraceID, &rs.SourceModel, &rs.SourceProvider,
				&rs.TargetModel, &rs.TargetProvider,
				&rs.SrcTokensIn, &rs.SrcTokensOut, &rs.SrcCostUSD, &rs.SrcLatencyMs,
				&rs.TgtTokensIn, &rs.TgtTokensOut, &rs.TgtCostUSD, &rs.TgtLatencyMs,
				&rs.Status, &rs.Error, &rs.CreatedAt); err != nil {
				continue
			}
			rs.CostDelta = rs.TgtCostUSD - rs.SrcCostUSD
			rs.SpeedDelta = rs.TgtLatencyMs - rs.SrcLatencyMs
			if rs.Error == "" {
				rs.Error = "" // omitempty
			}
			runs = append(runs, rs)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		replayJSON(w, 200, map[string]any{"runs": runs, "count": len(runs)})
	}
}

// handleReplayRun returns a single replay run detail.
// GET /api/replay/runs/{id}
func handleReplayRun(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var srcTraceID, srcModel, srcProv, tgtModel, tgtProv string
		var reqBody, srcResp, tgtResp, errMsg, status, createdAt string
		var srcTokIn, srcTokOut, srcDurMs, tgtTokIn, tgtTokOut, tgtDurMs int64
		var srcCost, tgtCost float64

		err := conn.QueryRow(`SELECT source_trace_id, source_model, source_provider,
			target_model, target_provider, request_body, source_response, target_response,
			source_tokens_in, source_tokens_out, source_cost_usd, source_duration_ms,
			target_tokens_in, target_tokens_out, target_cost_usd, target_duration_ms,
			status, COALESCE(error_message,''), created_at
			FROM replay_runs WHERE id = ?`, id).Scan(
			&srcTraceID, &srcModel, &srcProv, &tgtModel, &tgtProv,
			&reqBody, &srcResp, &tgtResp,
			&srcTokIn, &srcTokOut, &srcCost, &srcDurMs,
			&tgtTokIn, &tgtTokOut, &tgtCost, &tgtDurMs,
			&status, &errMsg, &createdAt)
		if err != nil {
			replayJSON(w, 404, map[string]string{"error": "replay run not found"})
			return
		}

		result := map[string]any{
			"id":         id,
			"status":     status,
			"created_at": createdAt,
			"request":    json.RawMessage(reqBody),
			"source": map[string]any{
				"trace_id":   srcTraceID,
				"model":      srcModel,
				"provider":   srcProv,
				"response":   srcResp,
				"tokens_in":  srcTokIn,
				"tokens_out": srcTokOut,
				"cost_usd":   srcCost,
				"latency_ms": srcDurMs,
			},
			"target": map[string]any{
				"model":      tgtModel,
				"provider":   tgtProv,
				"response":   tgtResp,
				"tokens_in":  tgtTokIn,
				"tokens_out": tgtTokOut,
				"cost_usd":   tgtCost,
				"latency_ms": tgtDurMs,
			},
			"delta": map[string]any{
				"cost_usd":    tgtCost - srcCost,
				"latency_ms":  tgtDurMs - srcDurMs,
				"total_tokens": (tgtTokIn + tgtTokOut) - (srcTokIn + srcTokOut),
			},
		}
		if errMsg != "" {
			result["error"] = errMsg
		}
		replayJSON(w, 200, result)
	}
}

// handleReplayCompare returns a side-by-side comparison for a replay run.
// GET /api/replay/compare/{id}
func handleReplayCompare(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var srcModel, srcProv, tgtModel, tgtProv string
		var srcResp, tgtResp, status string
		var srcTokIn, srcTokOut, srcDurMs, tgtTokIn, tgtTokOut, tgtDurMs int64
		var srcCost, tgtCost float64

		err := conn.QueryRow(`SELECT source_model, source_provider,
			target_model, target_provider, source_response, target_response,
			source_tokens_in, source_tokens_out, source_cost_usd, source_duration_ms,
			target_tokens_in, target_tokens_out, target_cost_usd, target_duration_ms, status
			FROM replay_runs WHERE id = ?`, id).Scan(
			&srcModel, &srcProv, &tgtModel, &tgtProv, &srcResp, &tgtResp,
			&srcTokIn, &srcTokOut, &srcCost, &srcDurMs,
			&tgtTokIn, &tgtTokOut, &tgtCost, &tgtDurMs, &status)
		if err != nil {
			replayJSON(w, 404, map[string]string{"error": "replay run not found"})
			return
		}

		// Determine winner in each category
		costWinner := srcModel
		if tgtCost < srcCost {
			costWinner = tgtModel
		}
		speedWinner := srcModel
		if tgtDurMs < srcDurMs {
			speedWinner = tgtModel
		}
		efficiencyWinner := srcModel
		srcTotal := srcTokIn + srcTokOut
		tgtTotal := tgtTokIn + tgtTokOut
		if tgtTotal < srcTotal {
			efficiencyWinner = tgtModel
		}

		replayJSON(w, 200, map[string]any{
			"id": id,
			"comparison": map[string]any{
				"left": map[string]any{
					"label":      fmt.Sprintf("%s (%s)", srcModel, srcProv),
					"response":   srcResp,
					"tokens_in":  srcTokIn,
					"tokens_out": srcTokOut,
					"cost_usd":   srcCost,
					"latency_ms": srcDurMs,
				},
				"right": map[string]any{
					"label":      fmt.Sprintf("%s (%s)", tgtModel, tgtProv),
					"response":   tgtResp,
					"tokens_in":  tgtTokIn,
					"tokens_out": tgtTokOut,
					"cost_usd":   tgtCost,
					"latency_ms": tgtDurMs,
				},
			},
			"winners": map[string]string{
				"cost":       costWinner,
				"speed":      speedWinner,
				"efficiency": efficiencyWinner,
			},
			"delta": map[string]any{
				"cost_usd":       tgtCost - srcCost,
				"cost_pct":       pctChange(srcCost, tgtCost),
				"latency_ms":     tgtDurMs - srcDurMs,
				"latency_pct":    pctChange(float64(srcDurMs), float64(tgtDurMs)),
				"tokens":         tgtTotal - srcTotal,
			},
		})
	}
}

// handleReplayStats returns aggregate replay statistics.
// GET /api/replay/stats
func handleReplayStats(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var totalRuns, okRuns, errorRuns int
		var avgCostDelta, avgLatencyDelta float64

		conn.QueryRow(`SELECT COUNT(*) FROM replay_runs`).Scan(&totalRuns)
		conn.QueryRow(`SELECT COUNT(*) FROM replay_runs WHERE status='ok'`).Scan(&okRuns)
		conn.QueryRow(`SELECT COUNT(*) FROM replay_runs WHERE status='error'`).Scan(&errorRuns)
		conn.QueryRow(`SELECT COALESCE(AVG(target_cost_usd - source_cost_usd), 0) FROM replay_runs WHERE status='ok'`).Scan(&avgCostDelta)
		conn.QueryRow(`SELECT COALESCE(AVG(target_duration_ms - source_duration_ms), 0) FROM replay_runs WHERE status='ok'`).Scan(&avgLatencyDelta)

		// Model leaderboard: which target model is cheapest/fastest on average
		type modelStat struct {
			Model      string  `json:"model"`
			Runs       int     `json:"runs"`
			AvgCost    float64 `json:"avg_cost_usd"`
			AvgLatency float64 `json:"avg_latency_ms"`
		}
		rows, _ := conn.Query(`SELECT target_model, COUNT(*), AVG(target_cost_usd), AVG(target_duration_ms)
			FROM replay_runs WHERE status='ok' GROUP BY target_model ORDER BY COUNT(*) DESC LIMIT 10`)
		var leaderboard []modelStat
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var ms modelStat
				if err := rows.Scan(&ms.Model, &ms.Runs, &ms.AvgCost, &ms.AvgLatency); err != nil {
					continue
				}
				leaderboard = append(leaderboard, ms)
			}
			if err := rows.Err(); err != nil {
				log.Printf("[db] rows iteration error: %v", err)
			}
		}
		if leaderboard == nil {
			leaderboard = []modelStat{}
		}

		// Available providers for replay
		available := make([]string, 0, len(GlobalProviders))
		for k := range GlobalProviders {
			available = append(available, k)
		}

		// Count replayable traces (have request_body)
		var replayableTraces int
		conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE COALESCE(request_body,'') != ''`).Scan(&replayableTraces)

		replayJSON(w, 200, map[string]any{
			"total_runs":          totalRuns,
			"successful":          okRuns,
			"errors":              errorRuns,
			"avg_cost_delta_usd":  avgCostDelta,
			"avg_latency_delta_ms": avgLatencyDelta,
			"model_leaderboard":   leaderboard,
			"available_providers": available,
			"replayable_traces":   replayableTraces,
		})
	}
}

func genReplayID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "rpl_" + hex.EncodeToString(b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func pctChange(old, new float64) float64 {
	if old == 0 {
		return 0
	}
	return ((new - old) / old) * 100
}

func replayJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
