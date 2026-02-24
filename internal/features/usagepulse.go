package features

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// UsageRecord holds metered usage for a single dimension key.
type UsageRecord struct {
	Requests    int64
	TokensIn    int64
	TokensOut   int64
	CostUSD     float64
	FirstSeen   time.Time
	LastSeen    time.Time
}

// DimensionUsage holds all usage records for a single dimension.
type DimensionUsage struct {
	mu      sync.RWMutex
	records map[string]*UsageRecord // key → usage
}

// UsagePulseManager manages multi-dimensional usage metering.
type UsagePulseManager struct {
	mu         sync.RWMutex
	dimensions map[string]*DimensionUsage // dimension name → usage
	caps       map[string]config.UsageCapRule // "dimension:key" → cap rule
	enabled    []string
	exportFmt  string
}

// NewUsagePulse creates a usage pulse meter from config.
func NewUsagePulse(cfg config.UsagePulseConfig) *UsagePulseManager {
	up := &UsagePulseManager{
		dimensions: make(map[string]*DimensionUsage),
		caps:       make(map[string]config.UsageCapRule),
		enabled:    cfg.Dimensions,
		exportFmt:  cfg.ExportFormat,
	}

	if up.exportFmt == "" {
		up.exportFmt = "json"
	}

	// Default dimensions if none specified
	if len(up.enabled) == 0 {
		up.enabled = []string{"user", "project"}
	}

	for _, dim := range up.enabled {
		up.dimensions[dim] = &DimensionUsage{
			records: make(map[string]*UsageRecord),
		}
	}

	for _, cap := range cfg.Caps {
		capKey := cap.Dimension + ":" + cap.Key
		up.caps[capKey] = cap
	}

	return up
}

// Record adds a usage event across all enabled dimensions.
func (up *UsagePulseManager) Record(dims map[string]string, tokensIn, tokensOut int, costUSD float64) {
	now := time.Now()

	for dimName, du := range up.dimensions {
		key, ok := dims[dimName]
		if !ok || key == "" {
			key = "unknown"
		}

		du.mu.Lock()
		rec, exists := du.records[key]
		if !exists {
			rec = &UsageRecord{FirstSeen: now}
			du.records[key] = rec
		}
		rec.Requests++
		rec.TokensIn += int64(tokensIn)
		rec.TokensOut += int64(tokensOut)
		rec.CostUSD += costUSD
		rec.LastSeen = now
		du.mu.Unlock()
	}
}

// CheckCap verifies if a request would exceed any dimension caps.
// Returns an error describing the exceeded cap, or nil if OK.
func (up *UsagePulseManager) CheckCap(dims map[string]string) error {
	today := time.Now().Format("2006-01-02")
	_ = today // used conceptually for daily tracking

	for dimName, du := range up.dimensions {
		key, ok := dims[dimName]
		if !ok || key == "" {
			continue
		}

		capKey := dimName + ":" + key
		cap, hasCap := up.caps[capKey]
		if !hasCap {
			// Check wildcard cap
			capKey = dimName + ":*"
			cap, hasCap = up.caps[capKey]
			if !hasCap {
				continue
			}
		}

		du.mu.RLock()
		rec := du.records[key]
		du.mu.RUnlock()

		if rec == nil {
			continue
		}

		if cap.Daily > 0 && rec.CostUSD >= cap.Daily {
			return fmt.Errorf("usagepulse: %s %q exceeded daily cap ($%.2f of $%.2f)",
				dimName, key, rec.CostUSD, cap.Daily)
		}
		if cap.Monthly > 0 && rec.CostUSD >= cap.Monthly {
			return fmt.Errorf("usagepulse: %s %q exceeded monthly cap ($%.2f of $%.2f)",
				dimName, key, rec.CostUSD, cap.Monthly)
		}
	}

	return nil
}

// GetUsage returns usage records for a dimension.
func (up *UsagePulseManager) GetUsage(dimension string) map[string]*UsageRecord {
	du, ok := up.dimensions[dimension]
	if !ok {
		return nil
	}

	du.mu.RLock()
	defer du.mu.RUnlock()

	result := make(map[string]*UsageRecord, len(du.records))
	for k, v := range du.records {
		copy := *v
		result[k] = &copy
	}
	return result
}

// Stats returns usage statistics across all dimensions.
func (up *UsagePulseManager) Stats() map[string]any {
	stats := make(map[string]any)
	for dimName, du := range up.dimensions {
		du.mu.RLock()
		dimStats := make([]map[string]any, 0, len(du.records))
		for key, rec := range du.records {
			dimStats = append(dimStats, map[string]any{
				"key":        key,
				"requests":   rec.Requests,
				"tokens_in":  rec.TokensIn,
				"tokens_out": rec.TokensOut,
				"cost_usd":   rec.CostUSD,
				"first_seen": rec.FirstSeen.Format(time.RFC3339),
				"last_seen":  rec.LastSeen.Format(time.RFC3339),
			})
		}
		du.mu.RUnlock()
		stats[dimName] = dimStats
	}
	return stats
}

// ExportCSV writes usage data for a dimension to a CSV file.
func (up *UsagePulseManager) ExportCSV(dimension, path string) error {
	records := up.GetUsage(dimension)
	if records == nil {
		return fmt.Errorf("unknown dimension: %s", dimension)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"key", "requests", "tokens_in", "tokens_out", "cost_usd", "first_seen", "last_seen"})

	for key, rec := range records {
		w.Write([]string{
			key,
			fmt.Sprintf("%d", rec.Requests),
			fmt.Sprintf("%d", rec.TokensIn),
			fmt.Sprintf("%d", rec.TokensOut),
			fmt.Sprintf("%.6f", rec.CostUSD),
			rec.FirstSeen.Format(time.RFC3339),
			rec.LastSeen.Format(time.RFC3339),
		})
	}
	return nil
}

// ExportJSON writes usage data for a dimension to a JSON file.
func (up *UsagePulseManager) ExportJSON(dimension, path string) error {
	records := up.GetUsage(dimension)
	if records == nil {
		return fmt.Errorf("unknown dimension: %s", dimension)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	type exportRecord struct {
		Key       string  `json:"key"`
		Requests  int64   `json:"requests"`
		TokensIn  int64   `json:"tokens_in"`
		TokensOut int64   `json:"tokens_out"`
		CostUSD   float64 `json:"cost_usd"`
		FirstSeen string  `json:"first_seen"`
		LastSeen  string  `json:"last_seen"`
	}

	var export []exportRecord
	for key, rec := range records {
		export = append(export, exportRecord{
			Key:       key,
			Requests:  rec.Requests,
			TokensIn:  rec.TokensIn,
			TokensOut: rec.TokensOut,
			CostUSD:   rec.CostUSD,
			FirstSeen: rec.FirstSeen.Format(time.RFC3339),
			LastSeen:  rec.LastSeen.Format(time.RFC3339),
		})
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(export)
}

// extractDimensions pulls dimension values from a request.
func extractDimensions(req *provider.Request) map[string]string {
	dims := map[string]string{
		"project": req.Project,
		"user":    req.UserID,
		"model":   req.Model,
	}

	// Extract custom dimensions from headers/extra
	if req.Extra != nil {
		if team, ok := req.Extra["_team"].(string); ok {
			dims["team"] = team
		}
		if feature, ok := req.Extra["_feature"].(string); ok {
			dims["feature"] = feature
		}
		if custom, ok := req.Extra["_dimension"].(string); ok {
			parts := strings.SplitN(custom, ":", 2)
			if len(parts) == 2 {
				dims[parts[0]] = parts[1]
			}
		}
	}

	// Also check for X-Team, X-Feature headers stored in Extra
	if team, ok := req.Extra["X-Team"].(string); ok {
		dims["team"] = team
	}
	if feature, ok := req.Extra["X-Feature"].(string); ok {
		dims["feature"] = feature
	}

	return dims
}

// UsagePulseMiddleware returns middleware that meters usage across dimensions.
func UsagePulseMiddleware(pulse *UsagePulseManager) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			dims := extractDimensions(req)

			// Pre-request: check caps
			if err := pulse.CheckCap(dims); err != nil {
				return nil, err
			}

			// Send request
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Post-request: record usage
			cost := provider.CalculateCost(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			pulse.Record(dims, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, cost)

			log.Printf("usagepulse: user=%s project=%s tokens=%d cost=$%.6f",
				dims["user"], dims["project"], resp.Usage.TotalTokens, cost)

			return resp, nil
		}
	}
}
