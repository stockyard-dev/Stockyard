package features

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ── AgentGuard ──────────────────────────────────────────────────────

// AgentGuardEvent records a session action.
type AgentGuardEvent struct {
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Action    string    `json:"action"` // allowed, killed, warned
	Reason    string    `json:"reason"`
	Calls     int       `json:"calls"`
	Cost      float64   `json:"cost"`
	Model     string    `json:"model"`
}

// AgentSession tracks an agent session's usage.
type AgentSession struct {
	ID        string
	Calls     int
	TotalCost float64
	StartTime time.Time
	Models    map[string]int
}

// AgentGuardState holds runtime state for agent safety rails.
type AgentGuardState struct {
	mu           sync.Mutex
	cfg          config.AgentGuardConfig
	sessions     map[string]*AgentSession
	recentEvents []AgentGuardEvent

	sessionsTracked atomic.Int64
	sessionsKilled  atomic.Int64
	callsMonitored  atomic.Int64
	callsBlocked    atomic.Int64
	totalCostSaved  atomic.Int64 // stored as microdollars
}

// NewAgentGuard creates a new agent guard.
func NewAgentGuard(cfg config.AgentGuardConfig) *AgentGuardState {
	return &AgentGuardState{
		cfg:          cfg,
		sessions:     make(map[string]*AgentSession),
		recentEvents: make([]AgentGuardEvent, 0, 200),
	}
}

// Stats returns agent guard statistics for the dashboard.
func (ag *AgentGuardState) Stats() map[string]any {
	ag.mu.Lock()
	events := make([]AgentGuardEvent, len(ag.recentEvents))
	copy(events, ag.recentEvents)
	activeSessions := len(ag.sessions)
	ag.mu.Unlock()

	return map[string]any{
		"sessions_tracked": ag.sessionsTracked.Load(),
		"sessions_killed":  ag.sessionsKilled.Load(),
		"calls_monitored":  ag.callsMonitored.Load(),
		"calls_blocked":    ag.callsBlocked.Load(),
		"active_sessions":  activeSessions,
		"cost_saved":       float64(ag.totalCostSaved.Load()) / 1_000_000,
		"recent_events":    events,
	}
}

func (ag *AgentGuardState) recordEvent(ev AgentGuardEvent) {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	if len(ag.recentEvents) >= 200 {
		ag.recentEvents = ag.recentEvents[1:]
	}
	ag.recentEvents = append(ag.recentEvents, ev)
}

// AgentGuardMiddleware returns middleware that enforces agent session limits.
func AgentGuardMiddleware(ag *AgentGuardState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ag.callsMonitored.Add(1)

			sessionID := req.UserID
			if sessionID == "" {
				sessionID = req.Project
			}
			if sessionID == "" {
				return next(ctx, req)
			}

			ag.mu.Lock()
			sess, ok := ag.sessions[sessionID]
			if !ok {
				sess = &AgentSession{
					ID:        sessionID,
					StartTime: time.Now(),
					Models:    make(map[string]int),
				}
				ag.sessions[sessionID] = sess
				ag.sessionsTracked.Add(1)
			}
			sess.Calls++
			sess.Models[req.Model]++
			currentCalls := sess.Calls
			currentCost := sess.TotalCost
			ag.mu.Unlock()

			// Check max calls
			if ag.cfg.MaxCalls > 0 && currentCalls > ag.cfg.MaxCalls {
				ag.callsBlocked.Add(1)
				ag.sessionsKilled.Add(1)
				ag.recordEvent(AgentGuardEvent{
					Timestamp: time.Now(), SessionID: sessionID,
					Action: "killed", Reason: "max_calls_exceeded",
					Calls: currentCalls, Cost: currentCost, Model: req.Model,
				})
				return nil, fmt.Errorf("agentguard: session %s exceeded max calls (%d/%d)", sessionID, currentCalls, ag.cfg.MaxCalls)
			}

			// Check max cost
			if ag.cfg.MaxCost > 0 && currentCost > ag.cfg.MaxCost {
				ag.callsBlocked.Add(1)
				ag.sessionsKilled.Add(1)
				ag.recordEvent(AgentGuardEvent{
					Timestamp: time.Now(), SessionID: sessionID,
					Action: "killed", Reason: "max_cost_exceeded",
					Calls: currentCalls, Cost: currentCost, Model: req.Model,
				})
				return nil, fmt.Errorf("agentguard: session %s exceeded cost limit ($%.2f/$%.2f)", sessionID, currentCost, ag.cfg.MaxCost)
			}

			// Check max duration
			if ag.cfg.MaxDuration.Duration > 0 {
				ag.mu.Lock()
				elapsed := time.Since(sess.StartTime)
				ag.mu.Unlock()
				if elapsed > ag.cfg.MaxDuration.Duration {
					ag.callsBlocked.Add(1)
					ag.sessionsKilled.Add(1)
					ag.recordEvent(AgentGuardEvent{
						Timestamp: time.Now(), SessionID: sessionID,
						Action: "killed", Reason: "max_duration_exceeded",
						Calls: currentCalls, Cost: currentCost, Model: req.Model,
					})
					return nil, fmt.Errorf("agentguard: session %s exceeded max duration (%s)", sessionID, ag.cfg.MaxDuration.Duration)
				}
			}

			resp, err := next(ctx, req)

			// Track cost after response
			if resp != nil && resp.Usage.TotalTokens > 0 {
				cost := provider.CalculateCost(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				ag.mu.Lock()
				if s, ok := ag.sessions[sessionID]; ok {
					s.TotalCost += cost
				}
				ag.mu.Unlock()
			}

			return resp, err
		}
	}
}
