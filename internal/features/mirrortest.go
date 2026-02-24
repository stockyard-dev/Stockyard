package features

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type MirrorTestEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	PrimaryModel  string    `json:"primary_model"`
	ShadowModel   string    `json:"shadow_model"`
	PrimaryLatency float64  `json:"primary_latency_ms"`
	ShadowLatency  float64  `json:"shadow_latency_ms"`
	Match         bool      `json:"match"`
}

type MirrorTestState struct {
	mu           sync.Mutex
	cfg          config.MirrorTestConfig
	recentEvents []MirrorTestEvent
	requestsMirrored atomic.Int64
	shadowSuccess    atomic.Int64
	shadowFailures   atomic.Int64
}

func NewMirrorTest(cfg config.MirrorTestConfig) *MirrorTestState {
	return &MirrorTestState{cfg: cfg, recentEvents: make([]MirrorTestEvent, 0, 200)}
}

func (mt *MirrorTestState) Stats() map[string]any {
	mt.mu.Lock()
	events := make([]MirrorTestEvent, len(mt.recentEvents))
	copy(events, mt.recentEvents)
	mt.mu.Unlock()
	return map[string]any{
		"requests_mirrored": mt.requestsMirrored.Load(), "shadow_success": mt.shadowSuccess.Load(),
		"shadow_failures": mt.shadowFailures.Load(), "shadow_model": mt.cfg.ShadowModel,
		"recent_events": events,
	}
}

func MirrorTestMiddleware(mt *MirrorTestState, providers map[string]provider.Provider) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			mt.requestsMirrored.Add(1)
			primaryStart := time.Now()
			resp, err := next(ctx, req)
			primaryLatency := float64(time.Since(primaryStart).Milliseconds())

			// Fire shadow request async (never affects primary response)
			if mt.cfg.ShadowModel != "" {
				go func() {
					shadowReq := &provider.Request{
						Model: mt.cfg.ShadowModel, Messages: req.Messages,
						Temperature: req.Temperature, MaxTokens: req.MaxTokens,
					}
					shadowStart := time.Now()
					provName := provider.ProviderForModel(mt.cfg.ShadowModel)
					p, ok := providers[provName]
					shadowLatency := 0.0
					success := false
					if ok {
						_, shadowErr := p.Send(context.Background(), shadowReq)
						shadowLatency = float64(time.Since(shadowStart).Milliseconds())
						success = shadowErr == nil
					}
					if success { mt.shadowSuccess.Add(1) } else { mt.shadowFailures.Add(1) }
					mt.mu.Lock()
					if len(mt.recentEvents) >= 200 { mt.recentEvents = mt.recentEvents[1:] }
					mt.recentEvents = append(mt.recentEvents, MirrorTestEvent{
						Timestamp: time.Now(), PrimaryModel: req.Model, ShadowModel: mt.cfg.ShadowModel,
						PrimaryLatency: primaryLatency, ShadowLatency: shadowLatency, Match: success,
					})
					mt.mu.Unlock()
					log.Printf("mirrortest: shadow %s latency=%.0fms success=%v", mt.cfg.ShadowModel, shadowLatency, success)
				}()
			}
			return resp, err
		}
	}
}
