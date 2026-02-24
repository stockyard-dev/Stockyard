package features

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type DriftWatchEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
	Metric    string    `json:"metric"`
	Baseline  float64   `json:"baseline"`
	Current   float64   `json:"current"`
	Drift     float64   `json:"drift_pct"`
}

type DriftWatchState struct {
	mu           sync.Mutex
	cfg          config.DriftWatchConfig
	baselines    map[string]map[string]float64 // model -> metric -> value
	recentEvents []DriftWatchEvent
	requestsTracked atomic.Int64
	driftsDetected  atomic.Int64
}

func NewDriftWatch(cfg config.DriftWatchConfig) *DriftWatchState {
	return &DriftWatchState{
		cfg: cfg, baselines: make(map[string]map[string]float64),
		recentEvents: make([]DriftWatchEvent, 0, 200),
	}
}

func (dw *DriftWatchState) Stats() map[string]any {
	dw.mu.Lock()
	events := make([]DriftWatchEvent, len(dw.recentEvents))
	copy(events, dw.recentEvents)
	dw.mu.Unlock()
	return map[string]any{
		"requests_tracked": dw.requestsTracked.Load(), "drifts_detected": dw.driftsDetected.Load(),
		"models_tracked": len(dw.baselines), "recent_events": events,
	}
}

func (dw *DriftWatchState) dwRecordEvent(ev DriftWatchEvent) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	if len(dw.recentEvents) >= 200 { dw.recentEvents = dw.recentEvents[1:] }
	dw.recentEvents = append(dw.recentEvents, ev)
}

func DriftWatchMiddleware(dw *DriftWatchState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			dw.requestsTracked.Add(1)
			latency := time.Since(start).Seconds()
			outputLen := 0
			for _, c := range resp.Choices { outputLen += len(c.Message.Content) }

			dw.mu.Lock()
			if _, ok := dw.baselines[req.Model]; !ok {
				dw.baselines[req.Model] = map[string]float64{"latency": latency, "output_len": float64(outputLen)}
			} else {
				bl := dw.baselines[req.Model]
				if bl["latency"] > 0 {
					drift := (latency - bl["latency"]) / bl["latency"] * 100
					if drift > 50 || drift < -50 {
						dw.driftsDetected.Add(1)
						dw.recentEvents = append(dw.recentEvents, DriftWatchEvent{
							Timestamp: time.Now(), Model: req.Model, Metric: "latency",
							Baseline: bl["latency"], Current: latency, Drift: drift,
						})
					}
				}
				// Update rolling baseline
				bl["latency"] = bl["latency"]*0.95 + latency*0.05
				bl["output_len"] = bl["output_len"]*0.95 + float64(outputLen)*0.05
			}
			dw.mu.Unlock()
			return resp, nil
		}
	}
}
