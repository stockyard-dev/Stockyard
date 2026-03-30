// Package engine — autopilot provides automatic cost-optimized model routing.
// It calibrates model quality via replay, then routes each request to the cheapest
// model that meets the configured quality threshold.
package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/platform"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// FreeTierDailyLimit is the max autopilot routing decisions per day on Community tier.
const FreeTierDailyLimit = 100

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func MigrateAutopilot(conn *sql.DB) {
	conn.Exec(`CREATE TABLE IF NOT EXISTS autopilot_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		enabled INTEGER DEFAULT 0,
		quality_threshold REAL DEFAULT 0.85,
		reference_model TEXT DEFAULT 'gpt-4o',
		candidate_models TEXT DEFAULT '[]',
		max_cost_usd REAL DEFAULT 0,
		mode TEXT DEFAULT 'cost',
		sample_size INTEGER DEFAULT 10,
		created_at TEXT DEFAULT (datetime('now')),
		updated_at TEXT DEFAULT (datetime('now'))
	)`)
	// Seed default config
	conn.Exec(`INSERT OR IGNORE INTO autopilot_config (id) VALUES (1)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS autopilot_model_scores (
		model TEXT PRIMARY KEY,
		provider TEXT NOT NULL,
		quality_score REAL DEFAULT 0,
		avg_cost_usd REAL DEFAULT 0,
		avg_latency_ms REAL DEFAULT 0,
		avg_tokens_out REAL DEFAULT 0,
		success_rate REAL DEFAULT 0,
		calibration_runs INTEGER DEFAULT 0,
		last_calibrated TEXT DEFAULT '',
		input_price_per_m REAL DEFAULT 0,
		output_price_per_m REAL DEFAULT 0
	)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS autopilot_decisions (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		requested_model TEXT NOT NULL,
		routed_model TEXT NOT NULL,
		routed_provider TEXT NOT NULL,
		reason TEXT NOT NULL,
		quality_score REAL DEFAULT 0,
		estimated_cost REAL DEFAULT 0,
		estimated_savings REAL DEFAULT 0,
		created_at TEXT DEFAULT (datetime('now'))
	)`)
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_ap_decisions_created ON autopilot_decisions(created_at)`)

	conn.Exec(`CREATE TABLE IF NOT EXISTS autopilot_calibration_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_trace_id TEXT NOT NULL,
		model TEXT NOT NULL,
		provider TEXT NOT NULL,
		quality_score REAL DEFAULT 0,
		cost_usd REAL DEFAULT 0,
		latency_ms INTEGER DEFAULT 0,
		tokens_out INTEGER DEFAULT 0,
		response_preview TEXT DEFAULT '',
		reference_preview TEXT DEFAULT '',
		status TEXT DEFAULT 'ok',
		created_at TEXT DEFAULT (datetime('now'))
	)`)
}

// ---------------------------------------------------------------------------
// Autopilot engine
// ---------------------------------------------------------------------------

// AutopilotConfig matches the DB row.
type AutopilotConfig struct {
	Enabled          bool     `json:"enabled"`
	QualityThreshold float64  `json:"quality_threshold"`
	ReferenceModel   string   `json:"reference_model"`
	CandidateModels  []string `json:"candidate_models"`
	MaxCostUSD       float64  `json:"max_cost_usd,omitempty"`
	Mode             string   `json:"mode"` // "cost" (cheapest) | "balanced" | "speed"
	SampleSize       int      `json:"sample_size"`
}

// ModelScore represents calibrated quality/cost data for one model.
type ModelScore struct {
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	QualityScore    float64 `json:"quality_score"`
	AvgCostUSD      float64 `json:"avg_cost_usd"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	AvgTokensOut    float64 `json:"avg_tokens_out"`
	SuccessRate     float64 `json:"success_rate"`
	CalibrationRuns int     `json:"calibration_runs"`
	LastCalibrated  string  `json:"last_calibrated"`
	InputPricePerM  float64 `json:"input_price_per_m"`
	OutputPricePerM float64 `json:"output_price_per_m"`
}

// Autopilot is the core routing engine.
type Autopilot struct {
	mu     sync.RWMutex
	conn   *sql.DB
	config AutopilotConfig
	scores map[string]ModelScore // keyed by model name

	// Free-tier daily cap
	tierWatcher  *platform.TierWatcher
	dailyCount   atomic.Int64
	dailyDate    atomic.Value // stores string "2006-01-02"
	dailySavings atomic.Int64 // stores savings in micro-dollars (x1e6) for precision
}

// NewAutopilot creates and loads an Autopilot from the database.
func NewAutopilot(conn *sql.DB, tw *platform.TierWatcher) *Autopilot {
	ap := &Autopilot{conn: conn, scores: make(map[string]ModelScore), tierWatcher: tw}
	ap.reload()

	// Seed daily counter from existing decisions today
	today := time.Now().UTC().Format("2006-01-02")
	ap.dailyDate.Store(today)
	var count int64
	var savings float64
	conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(estimated_savings),0) FROM autopilot_decisions WHERE created_at >= ?`, today+"T00:00:00Z").Scan(&count, &savings)
	ap.dailyCount.Store(count)
	ap.dailySavings.Store(int64(savings * 1e6))
	log.Printf("[autopilot] free-tier cap: %d/%d used today, savings=$%.4f", count, FreeTierDailyLimit, savings)

	return ap
}

// resetDailyIfNeeded checks if the date has rolled over and resets counters.
func (ap *Autopilot) resetDailyIfNeeded() {
	today := time.Now().UTC().Format("2006-01-02")
	stored, _ := ap.dailyDate.Load().(string)
	if stored != today {
		ap.dailyCount.Store(0)
		ap.dailySavings.Store(0)
		ap.dailyDate.Store(today)
		log.Printf("[autopilot] daily cap reset for %s", today)
	}
}

// DailyCapStatus returns current free-tier cap usage.
func (ap *Autopilot) DailyCapStatus() map[string]any {
	ap.resetDailyIfNeeded()

	tier := platform.TierCommunity
	if ap.tierWatcher != nil {
		tier = ap.tierWatcher.CurrentTier()
	}

	used := ap.dailyCount.Load()
	savingsMicro := ap.dailySavings.Load()
	savingsUSD := float64(savingsMicro) / 1e6
	capped := tier <= platform.TierCommunity && used >= FreeTierDailyLimit

	result := map[string]any{
		"tier":          platform.TierName(tier),
		"used_today":    used,
		"savings_today": savingsUSD,
		"capped":        capped,
	}

	if tier <= platform.TierCommunity {
		result["limit"] = FreeTierDailyLimit
		result["remaining"] = max(0, FreeTierDailyLimit-int(used))
		if capped {
			result["upgrade_cta"] = map[string]any{
				"message": fmt.Sprintf("Drover saved you $%.2f today. Upgrade to Pro for unlimited autopilot routing.", savingsUSD),
				"tier":    "Pro",
				"price":   "$99.99/mo",
				"url":     "/pricing/",
			}
		} else if used > 0 {
			result["progress_cta"] = map[string]any{
				"message": fmt.Sprintf("Drover has saved you $%.2f today (%d/%d free requests used).", savingsUSD, used, FreeTierDailyLimit),
				"tier":    "Pro",
				"price":   "$99.99/mo",
				"url":     "/pricing/",
			}
		}
	} else {
		result["limit"] = "unlimited"
		result["remaining"] = "unlimited"
	}

	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (ap *Autopilot) reload() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	var enabled int
	var threshold, maxCost float64
	var refModel, candidatesJSON, mode string
	var sampleSize int
	err := ap.conn.QueryRow(`SELECT enabled, quality_threshold, reference_model, candidate_models, max_cost_usd, mode, sample_size FROM autopilot_config WHERE id=1`).
		Scan(&enabled, &threshold, &refModel, &candidatesJSON, &maxCost, &mode, &sampleSize)
	if err != nil {
		return
	}
	ap.config = AutopilotConfig{
		Enabled:          enabled == 1,
		QualityThreshold: threshold,
		ReferenceModel:   refModel,
		MaxCostUSD:       maxCost,
		Mode:             mode,
		SampleSize:       sampleSize,
	}
	if err := json.Unmarshal([]byte(candidatesJSON), &ap.config.CandidateModels); err != nil {
		log.Printf("[autopilot] json parse error: %v", err)
	}

	// Load scores
	rows, err := ap.conn.Query(`SELECT model, provider, quality_score, avg_cost_usd, avg_latency_ms, avg_tokens_out, success_rate, calibration_runs, last_calibrated, input_price_per_m, output_price_per_m FROM autopilot_model_scores`)
	if err != nil {
		return
	}
	defer rows.Close()
	ap.scores = make(map[string]ModelScore)
	for rows.Next() {
		var ms ModelScore
		if err := rows.Scan(&ms.Model, &ms.Provider, &ms.QualityScore, &ms.AvgCostUSD, &ms.AvgLatencyMs, &ms.AvgTokensOut, &ms.SuccessRate, &ms.CalibrationRuns, &ms.LastCalibrated, &ms.InputPricePerM, &ms.OutputPricePerM); err != nil {
			continue
		}
		ap.scores[ms.Model] = ms
	}
}

// pickModel selects the optimal model for a request.
// Returns (model, provider, reason) or empty strings if no routing needed.
func (ap *Autopilot) pickModel(requestedModel string) (string, string, string, float64, float64) {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	if !ap.config.Enabled || len(ap.scores) == 0 {
		return "", "", "", 0, 0
	}

	// Only route if the requested model is the reference model (or "auto")
	if requestedModel != ap.config.ReferenceModel && requestedModel != "auto" && requestedModel != "autopilot" {
		return "", "", "", 0, 0
	}

	// Gather eligible candidates: quality >= threshold, success_rate > 0
	type candidate struct {
		ModelScore
		costScore float64
	}
	var eligible []candidate
	for _, ms := range ap.scores {
		if ms.QualityScore >= ap.config.QualityThreshold && ms.SuccessRate >= 0.8 && ms.CalibrationRuns >= 1 {
			// Check max cost constraint
			if ap.config.MaxCostUSD > 0 && ms.AvgCostUSD > ap.config.MaxCostUSD {
				continue
			}
			eligible = append(eligible, candidate{ModelScore: ms})
		}
	}

	if len(eligible) == 0 {
		return "", "", "no candidates meet quality threshold", 0, 0
	}

	// Sort by optimization mode
	switch ap.config.Mode {
	case "speed":
		sort.Slice(eligible, func(i, j int) bool {
			return eligible[i].AvgLatencyMs < eligible[j].AvgLatencyMs
		})
	case "balanced":
		// Score = quality * 0.4 + (1/cost) * 0.3 + (1/latency) * 0.3
		for i := range eligible {
			costInv := 1.0
			if eligible[i].AvgCostUSD > 0 {
				costInv = 1.0 / eligible[i].AvgCostUSD
			}
			latInv := 1.0
			if eligible[i].AvgLatencyMs > 0 {
				latInv = 1000.0 / eligible[i].AvgLatencyMs
			}
			eligible[i].costScore = eligible[i].QualityScore*0.4 + costInv*0.3 + latInv*0.3
		}
		sort.Slice(eligible, func(i, j int) bool {
			return eligible[i].costScore > eligible[j].costScore
		})
	default: // "cost" — cheapest first
		sort.Slice(eligible, func(i, j int) bool {
			return eligible[i].AvgCostUSD < eligible[j].AvgCostUSD
		})
	}

	best := eligible[0]

	// Calculate estimated savings vs reference
	var refCost float64
	if ref, ok := ap.scores[ap.config.ReferenceModel]; ok {
		refCost = ref.AvgCostUSD
	}
	savings := refCost - best.AvgCostUSD

	reason := fmt.Sprintf("autopilot:%s:q=%.2f,cost=$%.6f", ap.config.Mode, best.QualityScore, best.AvgCostUSD)
	return best.Model, best.Provider, reason, best.QualityScore, savings
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// AutopilotMiddleware returns a middleware that auto-routes to the optimal model.
// On Community tier, routing is capped at FreeTierDailyLimit per day.
// When capped, requests pass through un-routed (no error, just no savings).
func AutopilotMiddleware(ap *Autopilot) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			model, prov, reason, quality, savings := ap.pickModel(req.Model)
			if model == "" {
				return next(ctx, req)
			}

			// Free-tier daily cap enforcement
			ap.resetDailyIfNeeded()
			if ap.tierWatcher != nil && ap.tierWatcher.CurrentTier() <= platform.TierCommunity {
				count := ap.dailyCount.Load()
				if count >= FreeTierDailyLimit {
					// Cap reached — pass through without routing
					if req.Extra == nil {
						req.Extra = make(map[string]any)
					}
					savingsUSD := float64(ap.dailySavings.Load()) / 1e6
					req.Extra["_autopilot_cap_reached"] = true
					req.Extra["_autopilot_cap_message"] = fmt.Sprintf(
						"Drover saved you $%.2f today. Upgrade to Pro for unlimited autopilot routing.",
						savingsUSD)
					log.Printf("[autopilot] free-tier cap reached (%d/%d), passing through", count, FreeTierDailyLimit)
					go FireDroverCapReached(count, savingsUSD)
					return next(ctx, req)
				}
			}

			if req.Extra == nil {
				req.Extra = make(map[string]any)
			}
			req.Extra["_autopilot_original_model"] = req.Model
			req.Extra["_autopilot_routed_model"] = model
			req.Extra["_autopilot_reason"] = reason
			req.Extra["_autopilot_quality"] = quality

			req.Model = model
			if prov != "" {
				req.Provider = prov
			}

			log.Printf("[autopilot] %s → %s (%s) quality=%.2f savings=$%.6f",
				req.Extra["_autopilot_original_model"], model, reason, quality, savings)

			resp, err := next(ctx, req)

			// Increment daily counter + savings tracker
			ap.dailyCount.Add(1)
			ap.dailySavings.Add(int64(savings * 1e6))

			// Check savings milestones (fire upgrade events)
			totalSavingsUSD := float64(ap.dailySavings.Load()) / 1e6
			for _, milestone := range []float64{1.0, 5.0, 10.0, 50.0} {
				if totalSavingsUSD >= milestone {
					go FireDroverSavingsMilestone(totalSavingsUSD, milestone)
					break // only fire highest applicable milestone
				}
			}

			// Log the decision asynchronously
			go func() {
				decisionID := genReplayID() // reuse ID generator
				estCost := 0.0
				if resp != nil {
					estCost = float64(resp.Usage.PromptTokens)/1000*0.002 + float64(resp.Usage.CompletionTokens)/1000*0.006
				}
				ap.conn.Exec(`INSERT INTO autopilot_decisions (id, trace_id, requested_model, routed_model, routed_provider, reason, quality_score, estimated_cost, estimated_savings, created_at)
					VALUES (?,?,?,?,?,?,?,?,?,?)`,
					decisionID, "", req.Extra["_autopilot_original_model"], model, prov, reason, quality, estCost, savings, time.Now().UTC().Format(time.RFC3339))
			}()

			return resp, err
		}
	}
}

// ---------------------------------------------------------------------------
// Calibration — the brain
// ---------------------------------------------------------------------------

// CalibrateResult is returned from a calibration run.
type CalibrateResult struct {
	ModelsScored   int                    `json:"models_scored"`
	TracesUsed     int                    `json:"traces_used"`
	Duration       string                 `json:"duration"`
	Scores         map[string]ModelScore  `json:"scores"`
	Recommendation string                 `json:"recommendation"`
}

// Calibrate runs replay-based calibration against all candidate models.
func (ap *Autopilot) Calibrate(ctx context.Context) (*CalibrateResult, error) {
	ap.mu.RLock()
	cfg := ap.config
	ap.mu.RUnlock()

	if len(cfg.CandidateModels) == 0 {
		return nil, fmt.Errorf("no candidate models configured")
	}

	sampleSize := cfg.SampleSize
	if sampleSize <= 0 {
		sampleSize = 10
	}
	if sampleSize > 50 {
		sampleSize = 50
	}

	// Get recent traces with request bodies for calibration
	rows, err := ap.conn.Query(`SELECT id, model, provider, COALESCE(request_body,''), COALESCE(response_body,''),
		COALESCE(tokens_in,0), COALESCE(tokens_out,0), COALESCE(cost_usd,0), COALESCE(duration_ms,0)
		FROM observe_traces
		WHERE COALESCE(request_body,'') != '' AND status='ok'
		ORDER BY created_at DESC LIMIT ?`, sampleSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}
	defer rows.Close()

	type traceData struct {
		ID           string
		Model        string
		Provider     string
		RequestBody  string
		ResponseBody string
		TokensIn     int64
		TokensOut    int64
		CostUSD      float64
		DurationMs   int64
	}

	var traces []traceData
	for rows.Next() {
		var t traceData
		if err := rows.Scan(&t.ID, &t.Model, &t.Provider, &t.RequestBody, &t.ResponseBody,
			&t.TokensIn, &t.TokensOut, &t.CostUSD, &t.DurationMs); err != nil {
			continue
		}
		traces = append(traces, t)
	}
	if len(traces) == 0 {
		return nil, fmt.Errorf("no replayable traces found — send some proxy requests first")
	}

	start := time.Now()
	pricing := provider.ListPricing()

	// Score each candidate model
	scoreAcc := make(map[string]*struct {
		totalQuality  float64
		totalCost     float64
		totalLatency  float64
		totalTokens   float64
		successes     int
		attempts      int
	})

	for _, candidateModel := range cfg.CandidateModels {
		scoreAcc[candidateModel] = &struct {
			totalQuality  float64
			totalCost     float64
			totalLatency  float64
			totalTokens   float64
			successes     int
			attempts      int
		}{}
	}

	// Also score the reference model from existing trace data
	if _, exists := scoreAcc[cfg.ReferenceModel]; !exists {
		scoreAcc[cfg.ReferenceModel] = &struct {
			totalQuality  float64
			totalCost     float64
			totalLatency  float64
			totalTokens   float64
			successes     int
			attempts      int
		}{}
	}
	// The reference model gets perfect quality from its own traces
	for _, t := range traces {
		acc := scoreAcc[cfg.ReferenceModel]
		acc.totalQuality += 1.0
		acc.totalCost += t.CostUSD
		acc.totalLatency += float64(t.DurationMs)
		acc.totalTokens += float64(t.TokensOut)
		acc.successes++
		acc.attempts++
	}

	// Replay each trace against each candidate
	for _, candidateModel := range cfg.CandidateModels {
		if candidateModel == cfg.ReferenceModel {
			continue // Already scored from source data
		}

		candidateProvider := provider.ProviderForModel(candidateModel)
		if candidateProvider == "" {
			continue
		}
		p, ok := GlobalProviders[candidateProvider]
		if !ok {
			continue
		}

		acc := scoreAcc[candidateModel]

		for _, t := range traces {
			var parsedReq struct {
				Messages    []provider.Message `json:"messages"`
				Temperature *float64           `json:"temperature,omitempty"`
				MaxTokens   *int               `json:"max_tokens,omitempty"`
			}
			if json.Unmarshal([]byte(t.RequestBody), &parsedReq) != nil || len(parsedReq.Messages) == 0 {
				continue
			}

			acc.attempts++

			provReq := &provider.Request{
				Model:       candidateModel,
				Messages:    parsedReq.Messages,
				Temperature: parsedReq.Temperature,
				MaxTokens:   parsedReq.MaxTokens,
				Extra:       map[string]any{},
			}

			reqStart := time.Now()
			resp, sendErr := p.Send(ctx, provReq)
			latency := time.Since(reqStart)

			if sendErr != nil {
				// Log failed calibration
				ap.conn.Exec(`INSERT INTO autopilot_calibration_log (source_trace_id, model, provider, quality_score, cost_usd, latency_ms, tokens_out, status, created_at)
					VALUES (?,?,?,0,0,?,0,'error',?)`,
					t.ID, candidateModel, candidateProvider, latency.Milliseconds(), time.Now().UTC().Format(time.RFC3339))
				continue
			}

			var candidateResp string
			var tokOut int64
			if resp != nil && len(resp.Choices) > 0 {
				candidateResp = resp.Choices[0].Message.Content
				tokOut = int64(resp.Usage.CompletionTokens)
			}

			// Quality scoring: compare candidate response to reference
			quality := scoreQuality(t.ResponseBody, candidateResp, t.TokensOut, tokOut)

			// Real cost from pricing table
			var cost float64
			if mp, ok := pricing[candidateModel]; ok {
				inTok := int64(resp.Usage.PromptTokens)
				cost = float64(inTok)/1e6*mp.Input + float64(tokOut)/1e6*mp.Output
			} else {
				cost = float64(resp.Usage.PromptTokens)/1000*0.002 + float64(tokOut)/1000*0.006
			}

			acc.totalQuality += quality
			acc.totalCost += cost
			acc.totalLatency += float64(latency.Milliseconds())
			acc.totalTokens += float64(tokOut)
			acc.successes++

			// Log calibration detail
			ap.conn.Exec(`INSERT INTO autopilot_calibration_log (source_trace_id, model, provider, quality_score, cost_usd, latency_ms, tokens_out, response_preview, reference_preview, status, created_at)
				VALUES (?,?,?,?,?,?,?,?,?,'ok',?)`,
				t.ID, candidateModel, candidateProvider, quality, cost, latency.Milliseconds(), tokOut,
				truncate(candidateResp, 200), truncate(t.ResponseBody, 200), time.Now().UTC().Format(time.RFC3339))
		}
	}

	// Write scores to DB
	result := &CalibrateResult{
		TracesUsed: len(traces),
		Scores:     make(map[string]ModelScore),
	}

	for model, acc := range scoreAcc {
		if acc.attempts == 0 {
			continue
		}
		ms := ModelScore{
			Model:           model,
			Provider:        provider.ProviderForModel(model),
			CalibrationRuns: acc.successes,
			LastCalibrated:  time.Now().UTC().Format(time.RFC3339),
		}
		if acc.successes > 0 {
			ms.QualityScore = acc.totalQuality / float64(acc.successes)
			ms.AvgCostUSD = acc.totalCost / float64(acc.successes)
			ms.AvgLatencyMs = acc.totalLatency / float64(acc.successes)
			ms.AvgTokensOut = acc.totalTokens / float64(acc.successes)
		}
		ms.SuccessRate = float64(acc.successes) / float64(acc.attempts)

		// Add pricing info
		if mp, ok := pricing[model]; ok {
			ms.InputPricePerM = mp.Input
			ms.OutputPricePerM = mp.Output
		}

		ap.conn.Exec(`INSERT INTO autopilot_model_scores (model, provider, quality_score, avg_cost_usd, avg_latency_ms, avg_tokens_out, success_rate, calibration_runs, last_calibrated, input_price_per_m, output_price_per_m)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(model) DO UPDATE SET
				quality_score=excluded.quality_score, avg_cost_usd=excluded.avg_cost_usd,
				avg_latency_ms=excluded.avg_latency_ms, avg_tokens_out=excluded.avg_tokens_out,
				success_rate=excluded.success_rate, calibration_runs=excluded.calibration_runs,
				last_calibrated=excluded.last_calibrated, input_price_per_m=excluded.input_price_per_m,
				output_price_per_m=excluded.output_price_per_m`,
			ms.Model, ms.Provider, ms.QualityScore, ms.AvgCostUSD, ms.AvgLatencyMs, ms.AvgTokensOut,
			ms.SuccessRate, ms.CalibrationRuns, ms.LastCalibrated, ms.InputPricePerM, ms.OutputPricePerM)

		result.Scores[model] = ms
	}

	result.ModelsScored = len(result.Scores)
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	// Generate recommendation
	result.Recommendation = ap.recommend(result.Scores)

	// Reload scores into memory
	ap.reload()

	return result, nil
}

// scoreQuality computes a 0.0–1.0 quality score comparing candidate to reference.
func scoreQuality(refResponse, candidateResponse string, refTokens, candTokens int64) float64 {
	if candidateResponse == "" {
		return 0
	}

	// Factor 1: Length similarity (0-1)
	// Penalize responses that are too short or too long vs reference
	lenRatio := 1.0
	if len(refResponse) > 0 {
		ratio := float64(len(candidateResponse)) / float64(len(refResponse))
		if ratio > 1 {
			ratio = 1 / ratio // Normalize: 2x too long = 0.5
		}
		lenRatio = ratio
	}

	// Factor 2: Completion (did it produce a substantive response?)
	completionScore := 1.0
	if len(candidateResponse) < 5 {
		completionScore = 0.3
	}

	// Factor 3: Token efficiency
	tokenRatio := 1.0
	if refTokens > 0 && candTokens > 0 {
		ratio := float64(candTokens) / float64(refTokens)
		if ratio > 1 {
			ratio = 1 / ratio
		}
		tokenRatio = ratio
	}

	// Weighted combination
	quality := lenRatio*0.5 + completionScore*0.3 + tokenRatio*0.2

	// Clamp to [0, 1]
	if quality > 1 {
		quality = 1
	}
	if quality < 0 {
		quality = 0
	}
	return math.Round(quality*100) / 100
}

func (ap *Autopilot) recommend(scores map[string]ModelScore) string {
	ap.mu.RLock()
	threshold := ap.config.QualityThreshold
	ap.mu.RUnlock()

	type ranked struct {
		model string
		score ModelScore
	}
	var eligible []ranked
	for model, ms := range scores {
		if ms.QualityScore >= threshold && ms.SuccessRate >= 0.8 {
			eligible = append(eligible, ranked{model, ms})
		}
	}
	if len(eligible) == 0 {
		return "No models meet the quality threshold. Lower threshold or add more candidates."
	}
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].score.AvgCostUSD < eligible[j].score.AvgCostUSD
	})

	best := eligible[0]
	var refCost float64
	if ref, ok := scores[ap.config.ReferenceModel]; ok {
		refCost = ref.AvgCostUSD
	}
	if refCost > 0 && best.score.AvgCostUSD < refCost {
		savingsPct := (1.0 - best.score.AvgCostUSD/refCost) * 100
		return fmt.Sprintf("Route to %s — %.0f%% cheaper than %s with %.0f%% quality. Enable autopilot to save automatically.",
			best.model, savingsPct, ap.config.ReferenceModel, best.score.QualityScore*100)
	}
	return fmt.Sprintf("Best candidate: %s (quality=%.2f, cost=$%.6f/req)", best.model, best.score.QualityScore, best.score.AvgCostUSD)
}

// ---------------------------------------------------------------------------
// API Handlers
// ---------------------------------------------------------------------------

func RegisterAutopilotRoutes(mux *http.ServeMux, conn *sql.DB, ap *Autopilot) {
	mux.HandleFunc("GET /api/autopilot", handleAutopilotGet(ap))
	mux.HandleFunc("PUT /api/autopilot", handleAutopilotUpdate(conn, ap))
	mux.HandleFunc("POST /api/autopilot/calibrate", handleAutopilotCalibrate(ap))
	mux.HandleFunc("GET /api/autopilot/scores", handleAutopilotScores(ap))
	mux.HandleFunc("GET /api/autopilot/decisions", handleAutopilotDecisions(conn))
	mux.HandleFunc("GET /api/autopilot/stats", handleAutopilotStats(conn, ap))
	mux.HandleFunc("GET /api/autopilot/calibration-log", handleAutopilotCalibrationLog(conn))
	mux.HandleFunc("GET /api/autopilot/cap", handleAutopilotCap(ap))

	// Drover route aliases
	mux.HandleFunc("GET /api/drover/cap", handleAutopilotCap(ap))
	mux.HandleFunc("GET /api/drover/stats", handleAutopilotStats(conn, ap))
}

func handleAutopilotGet(ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ap.mu.RLock()
		cfg := ap.config
		scores := make(map[string]ModelScore, len(ap.scores))
		for k, v := range ap.scores {
			scores[k] = v
		}
		ap.mu.RUnlock()

		// Find current routed model
		currentRoute := cfg.ReferenceModel
		if cfg.Enabled {
			model, _, _, _, _ := ap.pickModel(cfg.ReferenceModel)
			if model != "" {
				currentRoute = model
			}
		}

		apJSON(w, 200, map[string]any{
			"config":        cfg,
			"current_route": currentRoute,
			"models_scored": len(scores),
			"scores":        scores,
		})
	}
}

func handleAutopilotUpdate(conn *sql.DB, ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Enabled          *bool    `json:"enabled,omitempty"`
			QualityThreshold *float64 `json:"quality_threshold,omitempty"`
			ReferenceModel   *string  `json:"reference_model,omitempty"`
			CandidateModels  []string `json:"candidate_models,omitempty"`
			MaxCostUSD       *float64 `json:"max_cost_usd,omitempty"`
			Mode             *string  `json:"mode,omitempty"`
			SampleSize       *int     `json:"sample_size,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apJSON(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}

		// Apply updates
		if req.Enabled != nil {
			v := 0
			if *req.Enabled {
				v = 1
			}
			conn.Exec(`UPDATE autopilot_config SET enabled=?, updated_at=datetime('now') WHERE id=1`, v)
		}
		if req.QualityThreshold != nil {
			if *req.QualityThreshold < 0 || *req.QualityThreshold > 1 {
				apJSON(w, 400, map[string]string{"error": "quality_threshold must be 0.0-1.0"})
				return
			}
			conn.Exec(`UPDATE autopilot_config SET quality_threshold=?, updated_at=datetime('now') WHERE id=1`, *req.QualityThreshold)
		}
		if req.ReferenceModel != nil {
			conn.Exec(`UPDATE autopilot_config SET reference_model=?, updated_at=datetime('now') WHERE id=1`, *req.ReferenceModel)
		}
		if req.CandidateModels != nil {
			j, _ := json.Marshal(req.CandidateModels)
			conn.Exec(`UPDATE autopilot_config SET candidate_models=?, updated_at=datetime('now') WHERE id=1`, string(j))
		}
		if req.MaxCostUSD != nil {
			conn.Exec(`UPDATE autopilot_config SET max_cost_usd=?, updated_at=datetime('now') WHERE id=1`, *req.MaxCostUSD)
		}
		if req.Mode != nil {
			valid := map[string]bool{"cost": true, "balanced": true, "speed": true}
			if !valid[*req.Mode] {
				apJSON(w, 400, map[string]string{"error": "mode must be cost, balanced, or speed"})
				return
			}
			conn.Exec(`UPDATE autopilot_config SET mode=?, updated_at=datetime('now') WHERE id=1`, *req.Mode)
		}
		if req.SampleSize != nil {
			if *req.SampleSize < 1 || *req.SampleSize > 50 {
				apJSON(w, 400, map[string]string{"error": "sample_size must be 1-50"})
				return
			}
			conn.Exec(`UPDATE autopilot_config SET sample_size=?, updated_at=datetime('now') WHERE id=1`, *req.SampleSize)
		}

		ap.reload()

		ap.mu.RLock()
		cfg := ap.config
		ap.mu.RUnlock()
		apJSON(w, 200, map[string]any{"status": "updated", "config": cfg})
	}
}

func handleAutopilotCalibrate(ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := ap.Calibrate(r.Context())
		if err != nil {
			apJSON(w, 422, map[string]string{"error": err.Error()})
			return
		}
		apJSON(w, 200, result)
	}
}

func handleAutopilotScores(ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ap.mu.RLock()
		scores := make([]ModelScore, 0, len(ap.scores))
		for _, ms := range ap.scores {
			scores = append(scores, ms)
		}
		threshold := ap.config.QualityThreshold
		ap.mu.RUnlock()

		// Sort by quality descending
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].QualityScore > scores[j].QualityScore
		})

		apJSON(w, 200, map[string]any{
			"scores":            scores,
			"quality_threshold": threshold,
			"count":             len(scores),
		})
	}
}

func handleAutopilotDecisions(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		rows, err := conn.Query(`SELECT id, trace_id, requested_model, routed_model, routed_provider, reason, quality_score, estimated_cost, estimated_savings, created_at
			FROM autopilot_decisions ORDER BY created_at DESC LIMIT ?`, limit)
		if err != nil {
			apJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		type decision struct {
			ID              string  `json:"id"`
			TraceID         string  `json:"trace_id"`
			RequestedModel  string  `json:"requested_model"`
			RoutedModel     string  `json:"routed_model"`
			RoutedProvider  string  `json:"routed_provider"`
			Reason          string  `json:"reason"`
			QualityScore    float64 `json:"quality_score"`
			EstimatedCost   float64 `json:"estimated_cost"`
			EstimatedSavings float64 `json:"estimated_savings"`
			CreatedAt       string  `json:"created_at"`
		}

		decisions := make([]decision, 0)
		for rows.Next() {
			var d decision
			if err := rows.Scan(&d.ID, &d.TraceID, &d.RequestedModel, &d.RoutedModel, &d.RoutedProvider,
				&d.Reason, &d.QualityScore, &d.EstimatedCost, &d.EstimatedSavings, &d.CreatedAt); err != nil {
				continue
			}
			decisions = append(decisions, d)
		}

		apJSON(w, 200, map[string]any{"decisions": decisions, "count": len(decisions)})
	}
}

func handleAutopilotStats(conn *sql.DB, ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var totalDecisions, todayDecisions int
		var totalSavings, todaySavings float64
		conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(estimated_savings),0) FROM autopilot_decisions`).Scan(&totalDecisions, &totalSavings)
		conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(estimated_savings),0) FROM autopilot_decisions WHERE created_at >= date('now')`).Scan(&todayDecisions, &todaySavings)

		// Route distribution
		type routeDist struct {
			Model string `json:"model"`
			Count int    `json:"count"`
			Pct   float64 `json:"pct"`
		}
		rows, _ := conn.Query(`SELECT routed_model, COUNT(*) FROM autopilot_decisions GROUP BY routed_model ORDER BY COUNT(*) DESC`)
		var dist []routeDist
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var rd routeDist
				if err := rows.Scan(&rd.Model, &rd.Count); err != nil {
					continue
				}
				if totalDecisions > 0 {
					rd.Pct = float64(rd.Count) / float64(totalDecisions) * 100
				}
				dist = append(dist, rd)
			}
		}
		if dist == nil {
			dist = []routeDist{}
		}

		// Calibration history
		var totalCalibrations int
		conn.QueryRow(`SELECT COUNT(DISTINCT created_at) FROM autopilot_calibration_log`).Scan(&totalCalibrations)

		ap.mu.RLock()
		cfg := ap.config
		ap.mu.RUnlock()

		apJSON(w, 200, map[string]any{
			"enabled":            cfg.Enabled,
			"mode":               cfg.Mode,
			"quality_threshold":  cfg.QualityThreshold,
			"reference_model":    cfg.ReferenceModel,
			"total_decisions":    totalDecisions,
			"today_decisions":    todayDecisions,
			"total_savings_usd":  totalSavings,
			"today_savings_usd":  todaySavings,
			"route_distribution": dist,
			"calibrations_run":   totalCalibrations,
			"daily_cap":          ap.DailyCapStatus(),
		})
	}
}

func handleAutopilotCalibrationLog(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		modelFilter := r.URL.Query().Get("model")

		query := `SELECT source_trace_id, model, provider, quality_score, cost_usd, latency_ms, tokens_out, COALESCE(response_preview,''), COALESCE(reference_preview,''), status, created_at FROM autopilot_calibration_log`
		args := []any{}
		if modelFilter != "" {
			query += ` WHERE model = ?`
			args = append(args, modelFilter)
		}
		query += ` ORDER BY created_at DESC LIMIT ?`
		args = append(args, limit)

		rows, err := conn.Query(query, args...)
		if err != nil {
			apJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		type logEntry struct {
			SourceTraceID   string  `json:"source_trace_id"`
			Model           string  `json:"model"`
			Provider        string  `json:"provider"`
			QualityScore    float64 `json:"quality_score"`
			CostUSD         float64 `json:"cost_usd"`
			LatencyMs       int64   `json:"latency_ms"`
			TokensOut       int64   `json:"tokens_out"`
			ResponsePreview string  `json:"response_preview"`
			ReferencePreview string `json:"reference_preview"`
			Status          string  `json:"status"`
			CreatedAt       string  `json:"created_at"`
		}

		entries := make([]logEntry, 0)
		for rows.Next() {
			var e logEntry
			if err := rows.Scan(&e.SourceTraceID, &e.Model, &e.Provider, &e.QualityScore,
				&e.CostUSD, &e.LatencyMs, &e.TokensOut, &e.ResponsePreview,
				&e.ReferencePreview, &e.Status, &e.CreatedAt); err != nil {
				continue
			}
			entries = append(entries, e)
		}

		apJSON(w, 200, map[string]any{"log": entries, "count": len(entries)})
	}
}

// Suppress unused import warning — strings is used in pickModel comments but
// keeping the import for future model name matching.
var _ = strings.Contains

func handleAutopilotCap(ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apJSON(w, 200, ap.DailyCapStatus())
	}
}

func apJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
