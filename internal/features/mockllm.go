package features

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// MockLLMEvent records a mock match for the dashboard.
type MockLLMEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	FixtureName string    `json:"fixture_name"`
	MatchType   string    `json:"match_type"`
	Model       string    `json:"model"`
	LatencyMs   int64     `json:"latency_ms"`
}

type compiledFixture struct {
	cfg   config.MockFixture
	regex *regexp.Regexp
}

// MockLLMState holds runtime state for the mock LLM server.
type MockLLMState struct {
	mu       sync.Mutex
	cfg      config.MockLLMConfig
	fixtures []compiledFixture
	recent   []MockLLMEvent

	requestsHandled atomic.Int64
	fixtureMatches  atomic.Int64
	defaultMatches  atomic.Int64
	regexMatches    atomic.Int64
	exactMatches    atomic.Int64
	containsMatches atomic.Int64
}

// NewMockLLM creates a new mock LLM server from config.
func NewMockLLM(cfg config.MockLLMConfig) *MockLLMState {
	state := &MockLLMState{
		cfg:    cfg,
		recent: make([]MockLLMEvent, 0, 64),
	}
	for _, f := range cfg.Fixtures {
		cf := compiledFixture{cfg: f}
		if f.MatchType == "regex" && f.Pattern != "" {
			if re, err := regexp.Compile(f.Pattern); err == nil {
				cf.regex = re
			}
		}
		state.fixtures = append(state.fixtures, cf)
	}
	return state
}

// Stats returns current metrics for the SSE dashboard.
func (m *MockLLMState) Stats() map[string]any {
	m.mu.Lock()
	recent := make([]MockLLMEvent, len(m.recent))
	copy(recent, m.recent)
	m.mu.Unlock()

	return map[string]any{
		"requests_handled": m.requestsHandled.Load(),
		"fixture_matches":  m.fixtureMatches.Load(),
		"default_matches":  m.defaultMatches.Load(),
		"exact_matches":    m.exactMatches.Load(),
		"contains_matches": m.containsMatches.Load(),
		"regex_matches":    m.regexMatches.Load(),
		"fixtures_loaded":  len(m.fixtures),
		"recent_events":    recent,
	}
}

// MockLLMMiddleware returns middleware that intercepts requests and returns fixture responses.
func MockLLMMiddleware(state *MockLLMState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			state.requestsHandled.Add(1)

			userContent := mlLastUserMsg(req)
			fixture, matchType := state.mlMatchFixture(userContent, req.Model)

			if fixture == nil {
				if state.cfg.Passthrough {
					return next(ctx, req)
				}
				state.defaultMatches.Add(1)
				return mlDefaultResponse(req, state.cfg.DefaultResponse), nil
			}

			// Simulate latency
			if fixture.DelayMs > 0 {
				time.Sleep(time.Duration(fixture.DelayMs) * time.Millisecond)
			}

			// Simulate errors
			if fixture.ErrorCode > 0 {
				return nil, fmt.Errorf("mock error (code %d): %s", fixture.ErrorCode, fixture.ErrorMessage)
			}

			switch matchType {
			case "exact":
				state.exactMatches.Add(1)
			case "contains":
				state.containsMatches.Add(1)
			case "regex":
				state.regexMatches.Add(1)
			}
			state.fixtureMatches.Add(1)

			state.mu.Lock()
			state.mlAddEvent(MockLLMEvent{
				Timestamp: time.Now(), FixtureName: fixture.Name,
				MatchType: matchType, Model: req.Model, LatencyMs: int64(fixture.DelayMs),
			})
			state.mu.Unlock()

			return mlBuildResponse(req, fixture), nil
		}
	}
}

func mlLastUserMsg(req *provider.Request) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}

func (m *MockLLMState) mlMatchFixture(content, model string) (*config.MockFixture, string) {
	for _, cf := range m.fixtures {
		f := cf.cfg
		if f.Model != "" && f.Model != model {
			continue
		}
		switch f.MatchType {
		case "exact":
			if strings.EqualFold(content, f.Pattern) {
				return &f, "exact"
			}
		case "contains":
			if strings.Contains(strings.ToLower(content), strings.ToLower(f.Pattern)) {
				return &f, "contains"
			}
		case "regex":
			if cf.regex != nil && cf.regex.MatchString(content) {
				return &f, "regex"
			}
		case "any", "":
			return &f, "any"
		}
	}
	return nil, ""
}

func mlBuildResponse(req *provider.Request, fixture *config.MockFixture) *provider.Response {
	content := fixture.Response
	if content == "" {
		content = "Mock response from fixture: " + fixture.Name
	}
	tokens := len(strings.Fields(content)) * 2
	promptTokens := 10
	for _, msg := range req.Messages {
		promptTokens += len(strings.Fields(msg.Content))
	}

	return &provider.Response{
		ID:     fmt.Sprintf("mock-%d", time.Now().UnixNano()),
		Object: "chat.completion",
		Model:  req.Model,
		Choices: []provider.Choice{
			{
				Index:        0,
				Message:      provider.Message{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
		Usage: provider.Usage{
			PromptTokens: promptTokens, CompletionTokens: tokens, TotalTokens: promptTokens + tokens,
		},
	}
}

func mlDefaultResponse(req *provider.Request, defaultResp string) *provider.Response {
	content := defaultResp
	if content == "" {
		content = "This is a mock response. No fixture matched your request."
	}
	return &provider.Response{
		ID: fmt.Sprintf("mock-default-%d", time.Now().UnixNano()), Object: "chat.completion", Model: req.Model,
		Choices: []provider.Choice{{
			Index: 0, Message: provider.Message{Role: "assistant", Content: content}, FinishReason: "stop",
		}},
		Usage: provider.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}
}

func (m *MockLLMState) mlAddEvent(evt MockLLMEvent) {
	m.recent = append(m.recent, evt)
	if len(m.recent) > 64 {
		m.recent = m.recent[len(m.recent)-64:]
	}
}
