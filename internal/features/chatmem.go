package features

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ChatMemEvent records a memory operation for the dashboard.
type ChatMemEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
	Action       string    `json:"action"`
	MessageCount int       `json:"message_count"`
}

// ChatMemSession holds conversation history for a single session.
type ChatMemSession struct {
	ID        string
	Messages  []provider.Message
	CreatedAt time.Time
	UpdatedAt time.Time
	Turns     int
}

// ChatMemState holds runtime state for conversation memory management.
type ChatMemState struct {
	mu       sync.Mutex
	cfg      config.ChatMemConfig
	sessions map[string]*ChatMemSession
	recent   []ChatMemEvent

	requestsProcessed atomic.Int64
	sessionsActive    atomic.Int64
	memoryInjections  atomic.Int64
	messagesStored    atomic.Int64
	evictions         atomic.Int64
}

// NewChatMem creates a new conversation memory manager.
func NewChatMem(cfg config.ChatMemConfig) *ChatMemState {
	return &ChatMemState{
		cfg:      cfg,
		sessions: make(map[string]*ChatMemSession),
		recent:   make([]ChatMemEvent, 0, 64),
	}
}

// Stats returns current metrics for the SSE dashboard.
func (c *ChatMemState) Stats() map[string]any {
	c.mu.Lock()
	recent := make([]ChatMemEvent, len(c.recent))
	copy(recent, c.recent)
	sessionCount := len(c.sessions)
	c.mu.Unlock()

	return map[string]any{
		"requests_processed": c.requestsProcessed.Load(),
		"sessions_active":    sessionCount,
		"memory_injections":  c.memoryInjections.Load(),
		"messages_stored":    c.messagesStored.Load(),
		"evictions":          c.evictions.Load(),
		"recent_events":      recent,
	}
}

// ChatMemMiddleware returns middleware that manages conversation memory.
func ChatMemMiddleware(state *ChatMemState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			state.requestsProcessed.Add(1)

			// Derive session ID from request metadata
			sessionID := cmSessionID(req)

			state.mu.Lock()
			session, exists := state.sessions[sessionID]
			if !exists {
				session = &ChatMemSession{
					ID: sessionID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
				}
				state.sessions[sessionID] = session
				state.sessionsActive.Add(1)
			}

			// Inject conversation memory into request
			if len(session.Messages) > 0 && state.cfg.InjectMemory {
				injected := cmInjectMemory(req, session, state.cfg)
				if injected > 0 {
					state.memoryInjections.Add(1)
					state.cmAddEvent(ChatMemEvent{
						Timestamp: time.Now(), SessionID: sessionID,
						Action: "inject", MessageCount: injected,
					})
				}
			}
			state.mu.Unlock()

			resp, err := next(ctx, req)

			// Store conversation turn
			state.mu.Lock()
			if s, ok := state.sessions[sessionID]; ok {
				for _, msg := range req.Messages {
					if msg.Role == "user" {
						s.Messages = append(s.Messages, msg)
						state.messagesStored.Add(1)
					}
				}
				if resp != nil && len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
					s.Messages = append(s.Messages, provider.Message{
						Role: "assistant", Content: resp.Choices[0].Message.Content,
					})
					state.messagesStored.Add(1)
				}
				s.UpdatedAt = time.Now()
				s.Turns++

				evicted := cmApplyWindow(s, state.cfg)
				if evicted > 0 {
					state.evictions.Add(int64(evicted))
					state.cmAddEvent(ChatMemEvent{
						Timestamp: time.Now(), SessionID: sessionID,
						Action: "evict", MessageCount: evicted,
					})
				}
			}
			state.cmPruneExpired()
			state.mu.Unlock()

			return resp, err
		}
	}
}

func cmSessionID(req *provider.Request) string {
	// Use UserID + Project if available
	if req.UserID != "" {
		if req.Project != "" {
			return req.UserID + ":" + req.Project
		}
		return req.UserID
	}
	// Hash from system prompt as fallback
	h := sha256.New()
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			h.Write([]byte(msg.Content))
			break
		}
	}
	h.Write([]byte("default"))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func cmInjectMemory(req *provider.Request, session *ChatMemSession, cfg config.ChatMemConfig) int {
	maxMsg := cfg.MaxMessages
	if maxMsg == 0 {
		maxMsg = 20
	}
	history := session.Messages
	if len(history) > maxMsg {
		history = history[len(history)-maxMsg:]
	}

	// Don't re-inject messages already present
	existing := make(map[string]bool)
	for _, msg := range req.Messages {
		existing[msg.Role+":"+msg.Content] = true
	}
	var toInject []provider.Message
	for _, msg := range history {
		if !existing[msg.Role+":"+msg.Content] {
			toInject = append(toInject, msg)
		}
	}
	if len(toInject) == 0 {
		return 0
	}

	// Insert after system message
	var result []provider.Message
	inserted := false
	for _, msg := range req.Messages {
		result = append(result, msg)
		if msg.Role == "system" && !inserted {
			result = append(result, toInject...)
			inserted = true
		}
	}
	if !inserted {
		result = append(toInject, result...)
	}
	req.Messages = result
	return len(toInject)
}

func cmApplyWindow(session *ChatMemSession, cfg config.ChatMemConfig) int {
	max := cfg.MaxMessages
	if max == 0 {
		max = 50
	}
	if len(session.Messages) <= max {
		return 0
	}
	evicted := len(session.Messages) - max

	switch cfg.Strategy {
	case "sliding_window":
		var sys, other []provider.Message
		for _, msg := range session.Messages {
			if msg.Role == "system" {
				sys = append(sys, msg)
			} else {
				other = append(other, msg)
			}
		}
		if len(other) > max {
			other = other[len(other)-max:]
		}
		session.Messages = append(sys, other...)
	case "importance":
		var sys, other []provider.Message
		for _, msg := range session.Messages {
			if msg.Role == "system" {
				sys = append(sys, msg)
			} else {
				other = append(other, msg)
			}
		}
		keep := max - len(sys)
		if keep < 0 {
			keep = 0
		}
		if len(other) > keep {
			other = other[len(other)-keep:]
		}
		session.Messages = append(sys, other...)
	default:
		session.Messages = session.Messages[evicted:]
	}
	return evicted
}

func (c *ChatMemState) cmAddEvent(evt ChatMemEvent) {
	c.recent = append(c.recent, evt)
	if len(c.recent) > 64 {
		c.recent = c.recent[len(c.recent)-64:]
	}
}

func (c *ChatMemState) cmPruneExpired() {
	ttl := c.cfg.SessionTTL.Duration
	if ttl == 0 {
		ttl = 1 * time.Hour
	}
	cutoff := time.Now().Add(-ttl)
	for id, s := range c.sessions {
		if s.UpdatedAt.Before(cutoff) {
			delete(c.sessions, id)
			log.Printf("chatmem: expired session %s (%d turns)", id, s.Turns)
		}
	}
}

func (c *ChatMemState) String() string {
	return fmt.Sprintf("ChatMem(sessions=%d strategy=%s)", len(c.sessions), c.cfg.Strategy)
}
