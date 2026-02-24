package features

import (
	"context"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// SpendConfig holds spend tracking configuration.
type SpendConfig struct {
	Counter     *tracker.SpendCounter
	Alerter     *Alerter           // nil = no alerts
	Caps        map[string]CapConfig // project → caps
	Broadcaster EventBroadcaster    // nil = no dashboard events
}

// SpendMiddleware returns middleware that tracks per-request spend.
// This runs AFTER the request completes, recording actual cost.
func SpendMiddleware(cfg SpendConfig) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Calculate actual cost from response
			var tokensIn, tokensOut int
			if resp.Usage.PromptTokens > 0 {
				tokensIn = resp.Usage.PromptTokens
				tokensOut = resp.Usage.CompletionTokens
			} else {
				tokensIn = tracker.CountInputTokens(req.Model, req.Messages)
				if len(resp.Choices) > 0 {
					tokensOut = tracker.CountOutputTokens(resp.Choices[0].Message.Content)
				}
			}

			cost := provider.CalculateCost(req.Model, tokensIn, tokensOut)

			// Update in-memory counter
			cfg.Counter.Add(req.Project, cost)

			// Check alert thresholds
			if cfg.Alerter != nil && cfg.Caps != nil {
				if capCfg, ok := cfg.Caps[req.Project]; ok {
					spend := cfg.Counter.Get(req.Project)
					if capCfg.DailyCap > 0 {
						cfg.Alerter.CheckAndAlert(req.Project, spend.Today, capCfg.DailyCap)
					}
					if capCfg.MonthlyCap > 0 {
						cfg.Alerter.CheckAndAlert(req.Project, spend.Month, capCfg.MonthlyCap)
					}
				}
			}

			// Broadcast spend update
			if cfg.Broadcaster != nil {
				spend := cfg.Counter.Get(req.Project)
				capVal := 0.0
				if c, ok := cfg.Caps[req.Project]; ok {
					capVal = c.DailyCap
				}
				cfg.Broadcaster.Send(map[string]any{
					"type":    "spend_update",
					"project": req.Project,
					"today":   spend.Today,
					"month":   spend.Month,
					"cap":     capVal,
				})
			}

			return resp, nil
		}
	}
}
