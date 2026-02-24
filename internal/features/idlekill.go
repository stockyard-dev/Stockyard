package features

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// IdleKillEvent records a killed request for the dashboard.
type IdleKillEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
	Model     string    `json:"model"`
	Duration  int64     `json:"duration_ms"`
	Tokens    int       `json:"tokens_at_kill"`
	CostEst   float64   `json:"cost_est"`
}

type ikTimedHash struct {
	t    time.Time
	hash string
}

// IdleKillState holds runtime state for runaway request termination.
type IdleKillState struct {
	mu     sync.Mutex
	cfg    config.IdleKillConfig
	recent []IdleKillEvent
	client *http.Client

	recentHashes []ikTimedHash

	requestsMonitored atomic.Int64
	requestsKilled    atomic.Int64
	killsByDuration   atomic.Int64
	killsByTokens     atomic.Int64
	killsByCost       atomic.Int64
	killsByLoop       atomic.Int64
	tokensSaved       atomic.Int64
	webhooksSent      atomic.Int64

	costSavedMu sync.Mutex
	costSaved   float64
}

// NewIdleKill creates a new idle kill monitor from config.
func NewIdleKill(cfg config.IdleKillConfig) *IdleKillState {
	return &IdleKillState{
		cfg:    cfg,
		recent: make([]IdleKillEvent, 0, 64),
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Stats returns current metrics for the SSE dashboard.
func (k *IdleKillState) Stats() map[string]any {
	k.mu.Lock()
	recent := make([]IdleKillEvent, len(k.recent))
	copy(recent, k.recent)
	k.mu.Unlock()

	k.costSavedMu.Lock()
	cs := k.costSaved
	k.costSavedMu.Unlock()

	return map[string]any{
		"requests_monitored": k.requestsMonitored.Load(),
		"requests_killed":    k.requestsKilled.Load(),
		"kills_by_duration":  k.killsByDuration.Load(),
		"kills_by_tokens":    k.killsByTokens.Load(),
		"kills_by_cost":      k.killsByCost.Load(),
		"kills_by_loop":      k.killsByLoop.Load(),
		"tokens_saved":       k.tokensSaved.Load(),
		"cost_saved":         cs,
		"webhooks_sent":      k.webhooksSent.Load(),
		"recent_kills":       recent,
	}
}

// IdleKillMiddleware returns middleware that kills runaway LLM requests.
func IdleKillMiddleware(state *IdleKillState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			state.requestsMonitored.Add(1)

			// Loop detection
			if state.cfg.LoopDetection {
				reqHash := ikHashRequest(req)
				if state.ikIsLoop(reqHash) {
					state.requestsKilled.Add(1)
					state.killsByLoop.Add(1)
					evt := IdleKillEvent{Timestamp: time.Now(), Reason: "loop_detected", Model: req.Model}
					state.mu.Lock()
					state.ikAddEvent(evt)
					state.mu.Unlock()
					go state.ikFireWebhook(evt)
					log.Printf("idlekill: loop detected model=%s — killed", req.Model)
					return nil, fmt.Errorf("idlekill: agent loop detected — request killed")
				}
				state.ikRecordHash(reqHash)
			}

			// Set max duration via context
			maxDur := state.cfg.MaxDuration.Duration
			if maxDur == 0 {
				maxDur = 120 * time.Second
			}
			killCtx, cancel := context.WithTimeout(ctx, maxDur)
			defer cancel()

			start := time.Now()
			resp, err := next(killCtx, req)
			duration := time.Since(start)

			// Check timeout
			if killCtx.Err() == context.DeadlineExceeded {
				state.requestsKilled.Add(1)
				state.killsByDuration.Add(1)
				evt := IdleKillEvent{
					Timestamp: time.Now(), Reason: "duration",
					Model: req.Model, Duration: duration.Milliseconds(),
				}
				state.mu.Lock()
				state.ikAddEvent(evt)
				state.mu.Unlock()
				go state.ikFireWebhook(evt)
				log.Printf("idlekill: killed model=%s after %s (max: %s)", req.Model, duration, maxDur)
				return nil, fmt.Errorf("idlekill: request killed — exceeded max duration %s", maxDur)
			}

			// Check token threshold
			if resp != nil && state.cfg.MaxTokensPerRequest > 0 && resp.Usage.TotalTokens > state.cfg.MaxTokensPerRequest {
				state.requestsKilled.Add(1)
				state.killsByTokens.Add(1)
				saved := resp.Usage.TotalTokens - state.cfg.MaxTokensPerRequest
				state.tokensSaved.Add(int64(saved))
				evt := IdleKillEvent{
					Timestamp: time.Now(), Reason: "tokens", Model: req.Model,
					Duration: duration.Milliseconds(), Tokens: resp.Usage.TotalTokens,
				}
				state.mu.Lock()
				state.ikAddEvent(evt)
				state.mu.Unlock()
				go state.ikFireWebhook(evt)
				log.Printf("idlekill: killed model=%s tokens=%d (max: %d)", req.Model, resp.Usage.TotalTokens, state.cfg.MaxTokensPerRequest)
				return nil, fmt.Errorf("idlekill: request killed — exceeded max tokens %d", state.cfg.MaxTokensPerRequest)
			}

			// Check cost threshold
			if resp != nil && state.cfg.MaxCostPerRequest > 0 {
				cost := float64(resp.Usage.TotalTokens) * 0.00001
				if cost > state.cfg.MaxCostPerRequest {
					state.requestsKilled.Add(1)
					state.killsByCost.Add(1)
					state.costSavedMu.Lock()
					state.costSaved += cost - state.cfg.MaxCostPerRequest
					state.costSavedMu.Unlock()
					evt := IdleKillEvent{
						Timestamp: time.Now(), Reason: "cost", Model: req.Model,
						Duration: duration.Milliseconds(), Tokens: resp.Usage.TotalTokens, CostEst: cost,
					}
					state.mu.Lock()
					state.ikAddEvent(evt)
					state.mu.Unlock()
					go state.ikFireWebhook(evt)
					log.Printf("idlekill: killed model=%s cost=$%.4f (max: $%.4f)", req.Model, cost, state.cfg.MaxCostPerRequest)
					return nil, fmt.Errorf("idlekill: request killed — exceeded max cost $%.4f", state.cfg.MaxCostPerRequest)
				}
			}

			return resp, err
		}
	}
}

func ikHashRequest(req *provider.Request) string {
	content := req.Model + ":"
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			content += req.Messages[i].Content
			break
		}
	}
	return fmt.Sprintf("%x", content)
}

func (k *IdleKillState) ikIsLoop(hash string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	window := k.cfg.LoopWindow.Duration
	if window == 0 {
		window = 60 * time.Second
	}
	threshold := k.cfg.LoopThreshold
	if threshold == 0 {
		threshold = 5
	}

	cutoff := time.Now().Add(-window)
	count := 0
	for _, h := range k.recentHashes {
		if h.t.After(cutoff) && h.hash == hash {
			count++
		}
	}
	return count >= threshold
}

func (k *IdleKillState) ikRecordHash(hash string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.recentHashes = append(k.recentHashes, ikTimedHash{t: time.Now(), hash: hash})
	cutoff := time.Now().Add(-5 * time.Minute)
	i := 0
	for _, h := range k.recentHashes {
		if h.t.After(cutoff) {
			k.recentHashes[i] = h
			i++
		}
	}
	k.recentHashes = k.recentHashes[:i]
}

func (k *IdleKillState) ikFireWebhook(evt IdleKillEvent) {
	if k.cfg.WebhookURL == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event": "request_killed", "reason": evt.Reason, "model": evt.Model,
		"duration_ms": evt.Duration, "tokens": evt.Tokens, "cost_est": evt.CostEst,
		"timestamp": evt.Timestamp.Format(time.RFC3339),
	})
	r, err := http.NewRequest("POST", k.cfg.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "Stockyard-IdleKill/1.0")
	resp, err := k.client.Do(r)
	if err != nil {
		log.Printf("idlekill: webhook failed: %v", err)
		return
	}
	resp.Body.Close()
	k.webhooksSent.Add(1)
}

func (k *IdleKillState) ikAddEvent(evt IdleKillEvent) {
	k.recent = append(k.recent, evt)
	if len(k.recent) > 64 {
		k.recent = k.recent[len(k.recent)-64:]
	}
}
