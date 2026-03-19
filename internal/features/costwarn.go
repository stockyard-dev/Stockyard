package features

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// CostWarnConfig holds the per-request cost warning/blocking thresholds.
type CostWarnConfig struct {
	WarnCents  int64 // warn if estimated cost exceeds this (cents)
	BlockCents int64 // block if estimated cost exceeds this (cents)
}

// CostWarnFromDB loads costwarn config from proxy_modules config_json.
func CostWarnFromDB(conn *sql.DB) CostWarnConfig {
	cfg := CostWarnConfig{WarnCents: 50, BlockCents: 500} // defaults: warn $0.50, block $5.00
	var raw string
	err := conn.QueryRow("SELECT config_json FROM proxy_modules WHERE name = 'costwarn'").Scan(&raw)
	if err != nil || raw == "" || raw == "{}" {
		return cfg
	}
	var parsed struct {
		WarnCents  *int64 `json:"warn_cents"`
		BlockCents *int64 `json:"block_cents"`
	}
	if json.Unmarshal([]byte(raw), &parsed) == nil {
		if parsed.WarnCents != nil {
			cfg.WarnCents = *parsed.WarnCents
		}
		if parsed.BlockCents != nil {
			cfg.BlockCents = *parsed.BlockCents
		}
	}
	return cfg
}

// CostWarnMiddleware estimates the cost of a request before sending it.
// If the estimated input cost exceeds the block threshold, the request is rejected.
// If it exceeds the warn threshold, a warning header is added but the request proceeds.
func CostWarnMiddleware(conn *sql.DB) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			cfg := CostWarnFromDB(conn)

			// Estimate input tokens from message content (~1 token per 4 chars)
			var totalChars int
			for _, m := range req.Messages {
				totalChars += len(m.Content)
			}
			estInputTokens := totalChars / 4
			if estInputTokens < 1 {
				estInputTokens = 1
			}

			// Estimate output tokens as max_tokens or 2048 default
			estOutputTokens := 2048
			if req.MaxTokens != nil && *req.MaxTokens > 0 {
				estOutputTokens = *req.MaxTokens
			}

			estCostUSD := provider.CalculateCost(req.Model, estInputTokens, estOutputTokens)
			estCostCents := int64(estCostUSD * 100)

			// Block if over block threshold
			if cfg.BlockCents > 0 && estCostCents >= cfg.BlockCents {
				log.Printf("[costwarn] BLOCKED request: model=%s est_input=%d est_output=%d est_cost=$%.4f block_threshold=$%.2f",
					req.Model, estInputTokens, estOutputTokens, estCostUSD, float64(cfg.BlockCents)/100)
				return nil, fmt.Errorf("costwarn: estimated cost $%.4f exceeds block threshold $%.2f", estCostUSD, float64(cfg.BlockCents)/100)
			}

			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Warn header if over warn threshold
			if cfg.WarnCents > 0 && estCostCents >= cfg.WarnCents {
				if resp.Meta == nil {
					resp.Meta = make(map[string]string)
				}
				resp.Meta["X-Stockyard-Cost-Warning"] = fmt.Sprintf("estimated $%.4f exceeds warn threshold $%.2f", estCostUSD, float64(cfg.WarnCents)/100)
				log.Printf("[costwarn] WARNING: model=%s est_cost=$%.4f warn_threshold=$%.2f",
					req.Model, estCostUSD, float64(cfg.WarnCents)/100)
			}

			return resp, nil
		}
	}
}
