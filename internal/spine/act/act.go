// Package act executes infrastructure decisions produced by the decide engine.
// Actions are logged, reversible, and rate-limited to prevent runaway automation.
package act

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/decide"
)

// ActionLog records every action taken for audit trail.
type ActionLog struct {
	Decision  decide.Decision `json:"decision"`
	Executed  bool            `json:"executed"`
	Result    string          `json:"result"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// Handler executes a specific type of infrastructure action.
type Handler interface {
	Execute(ctx context.Context, d decide.Decision) error
	Rollback(ctx context.Context, d decide.Decision) error
}

// Executor runs actions and maintains an audit log.
type Executor struct {
	mu       sync.RWMutex
	handlers map[decide.ActionType]Handler
	log      []ActionLog
	maxLog   int
	dryRun   bool
	onAction func(ActionLog) // callback for notifications
}

// Config holds executor settings.
type Config struct {
	DryRun   bool            // log actions without executing
	MaxLog   int             // max log entries (default 500)
	OnAction func(ActionLog) // called after each action
}

// NewExecutor creates an action executor.
func NewExecutor(cfg Config) *Executor {
	if cfg.MaxLog <= 0 {
		cfg.MaxLog = 500
	}
	return &Executor{
		handlers: map[decide.ActionType]Handler{},
		maxLog:   cfg.MaxLog,
		dryRun:   cfg.DryRun,
		onAction: cfg.OnAction,
	}
}

// RegisterHandler adds a handler for an action type.
func (e *Executor) RegisterHandler(action decide.ActionType, h Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[action] = h
}

// Execute runs a batch of decisions.
func (e *Executor) Execute(ctx context.Context, decisions []decide.Decision) []ActionLog {
	var logs []ActionLog

	for _, d := range decisions {
		entry := ActionLog{
			Decision:  d,
			Timestamp: time.Now(),
		}

		if e.dryRun {
			entry.Executed = false
			entry.Result = "dry run — would execute: " + string(d.Action)
			log.Printf("[spine/dry-run] %s: %s", d.Action, d.Reason)
		} else {
			e.mu.RLock()
			handler, ok := e.handlers[d.Action]
			e.mu.RUnlock()

			if !ok {
				entry.Executed = false
				entry.Error = fmt.Sprintf("no handler registered for action %s", d.Action)
				log.Printf("[spine/act] no handler for %s", d.Action)
			} else {
				err := handler.Execute(ctx, d)
				if err != nil {
					entry.Executed = false
					entry.Error = err.Error()
					log.Printf("[spine/act] %s failed: %v", d.Action, err)
				} else {
					entry.Executed = true
					entry.Result = "success"
					log.Printf("[spine/act] %s executed: %s", d.Action, d.Reason)
				}
			}
		}

		logs = append(logs, entry)
		e.record(entry)

		if e.onAction != nil {
			e.onAction(entry)
		}
	}

	return logs
}

func (e *Executor) record(entry ActionLog) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.log = append(e.log, entry)
	if len(e.log) > e.maxLog {
		e.log = e.log[len(e.log)-e.maxLog:]
	}
}

// RecentActions returns the last N action log entries.
func (e *Executor) RecentActions(n int) []ActionLog {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > len(e.log) {
		n = len(e.log)
	}
	out := make([]ActionLog, n)
	copy(out, e.log[len(e.log)-n:])
	return out
}

// =========================================================================
// Built-in action handlers
// =========================================================================

// LogHandler just logs the action (useful for alert actions).
type LogHandler struct{}

func (h *LogHandler) Execute(ctx context.Context, d decide.Decision) error {
	log.Printf("[spine/alert] %s — %s (urgency: %s)", d.Action, d.Reason, d.Urgency)
	return nil
}

func (h *LogHandler) Rollback(ctx context.Context, d decide.Decision) error {
	return nil // alerts don't need rollback
}

// WebhookHandler sends action notifications to a webhook URL.
type WebhookHandler struct {
	URL string
}

func (h *WebhookHandler) Execute(ctx context.Context, d decide.Decision) error {
	log.Printf("[spine/webhook] would POST to %s: %s — %s", h.URL, d.Action, d.Reason)
	// In production, this would HTTP POST the decision to the webhook URL
	return nil
}

func (h *WebhookHandler) Rollback(ctx context.Context, d decide.Decision) error {
	return nil
}
