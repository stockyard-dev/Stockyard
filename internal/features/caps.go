// Package features implements the pluggable feature layer for Stockyard products.
package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// CapConfig defines spend cap settings.
type CapConfig struct {
	DailyCap  float64 // USD, 0 = unlimited
	MonthlyCap float64 // USD, 0 = unlimited
	SoftCap    bool    // If true, allow but alert; if false, hard block
}

// CapError is returned when a spend cap is exceeded.
type CapError struct {
	Cap     float64   `json:"cap"`
	Spent   float64   `json:"spent"`
	Period  string    `json:"period"` // "daily" or "monthly"
	ResetsAt time.Time `json:"resets_at"`
}

func (e *CapError) Error() string {
	return fmt.Sprintf("%s cap exceeded: spent $%.4f of $%.2f cap", e.Period, e.Spent, e.Cap)
}

// WriteCapError writes a 429 response for cap exceeded.
func WriteCapError(w http.ResponseWriter, capErr *CapError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]any{
		"error":     "cap_exceeded",
		"cap":       capErr.Cap,
		"spent":     capErr.Spent,
		"period":    capErr.Period,
		"resets_at": capErr.ResetsAt.Format(time.RFC3339),
	})
}

// CapsMiddleware returns middleware that enforces spend caps.
func CapsMiddleware(caps map[string]CapConfig, counter *tracker.SpendCounter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			capCfg, ok := caps[req.Project]
			if !ok {
				capCfg = caps["default"]
			}

			spend := counter.Get(req.Project)

			// Check daily cap
			if capCfg.DailyCap > 0 {
				estimated := tracker.EstimateRequestCost(req.Model, req.Messages)
				if spend.Today+estimated > capCfg.DailyCap && !capCfg.SoftCap {
					return nil, &CapError{
						Cap:     capCfg.DailyCap,
						Spent:   spend.Today,
						Period:  "daily",
						ResetsAt: nextMidnight(),
					}
				}
			}

			// Check monthly cap
			if capCfg.MonthlyCap > 0 {
				estimated := tracker.EstimateRequestCost(req.Model, req.Messages)
				if spend.Month+estimated > capCfg.MonthlyCap && !capCfg.SoftCap {
					return nil, &CapError{
						Cap:     capCfg.MonthlyCap,
						Spent:   spend.Month,
						Period:  "monthly",
						ResetsAt: nextMonthStart(),
					}
				}
			}

			return next(ctx, req)
		}
	}
}

func nextMidnight() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

func nextMonthStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
}
