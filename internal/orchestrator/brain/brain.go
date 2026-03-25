// Package brain is the LLM-powered decision layer for the orchestrator.
// When predefined workflows don't cover a situation, the brain decides
// which products to invoke and in what order.
package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/hub"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Decision is the brain's response to a situation.
type Decision struct {
	Situation   string       `json:"situation"`
	Analysis    string       `json:"analysis"`
	Plan        []Action     `json:"plan"`
	Confidence  float64      `json:"confidence"`
	Reasoning   string       `json:"reasoning"`
	Timestamp   time.Time    `json:"timestamp"`
	LatencyMs   int          `json:"latency_ms"`
}

// Action is a single step the brain recommends.
type Action struct {
	ProductID   string         `json:"product_id"`
	Command     string         `json:"command"`
	Priority    int            `json:"priority"` // 1=immediate, 2=soon, 3=background
	Parameters  map[string]any `json:"parameters,omitempty"`
	Reason      string         `json:"reason"`
}

// Brain decides what to do in novel situations.
type Brain struct {
	llm   provider.Provider
	model string
	hub   *hub.Hub
	eventBus *bus.Bus
}

// New creates a brain.
func New(llm provider.Provider, model string, h *hub.Hub, eventBus *bus.Bus) *Brain {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Brain{llm: llm, model: model, hub: h, eventBus: eventBus}
}

// Decide analyzes a situation and returns a plan of action.
func (b *Brain) Decide(ctx context.Context, situation string, recentEvents []bus.Event) (*Decision, error) {
	if b.llm == nil {
		return nil, fmt.Errorf("no LLM configured")
	}

	start := time.Now()
	prompt := b.buildPrompt(situation, recentEvents)

	temp := 0.3
	maxTokens := 1500
	resp, err := b.llm.Send(ctx, &provider.Request{
		Model:       b.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("brain LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision Decision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		decision = Decision{
			Analysis:   raw,
			Confidence: 0.4,
			Reasoning:  "Unstructured LLM response",
		}
	}

	decision.Situation = situation
	decision.Timestamp = time.Now()
	decision.LatencyMs = int(time.Since(start).Milliseconds())

	return &decision, nil
}

// ExecuteDecision runs the brain's plan by emitting events for each action.
func (b *Brain) ExecuteDecision(decision *Decision) {
	for _, action := range decision.Plan {
		data := map[string]any{
			"command":    action.Command,
			"product":    action.ProductID,
			"reason":     action.Reason,
			"brain_conf": decision.Confidence,
		}
		for k, v := range action.Parameters {
			data[k] = v
		}

		// Emit as a decision event
		b.eventBus.Emit(bus.Event{
			Type:   bus.EventDecisionMade,
			Source: "brain",
			Data:   data,
		})
	}
}

func (b *Brain) buildPrompt(situation string, recentEvents []bus.Event) string {
	var sb strings.Builder

	sb.WriteString("You are the decision engine for Stockyard, an autonomous software platform.\n")
	sb.WriteString("Given a situation, decide which products to invoke and in what order.\n\n")

	sb.WriteString("Respond with ONLY JSON:\n")
	sb.WriteString(`{"analysis":"what's happening","plan":[{"product_id":"...","command":"...","priority":1,"reason":"..."}],"confidence":0.0-1.0,"reasoning":"why this plan"}`)
	sb.WriteString("\n\n")

	// Available products
	sb.WriteString("AVAILABLE PRODUCTS:\n")
	for _, p := range b.hub.All() {
		sb.WriteString(fmt.Sprintf("  %s (%s): %s\n", p.ID, p.Category, p.Description))
		sb.WriteString(fmt.Sprintf("    handles: %s\n", strings.Join(p.Capabilities, ", ")))
		sb.WriteString(fmt.Sprintf("    emits: %s\n", strings.Join(p.Emits, ", ")))
	}

	// Recent events for context
	if len(recentEvents) > 0 {
		sb.WriteString("\nRECENT EVENTS:\n")
		limit := 20
		if len(recentEvents) < limit {
			limit = len(recentEvents)
		}
		for _, e := range recentEvents[len(recentEvents)-limit:] {
			sb.WriteString(fmt.Sprintf("  [%s] %s from %s\n", e.Timestamp.Format("15:04:05"), e.Type, e.Source))
		}
	}

	sb.WriteString("\nSITUATION: ")
	sb.WriteString(situation)
	sb.WriteString("\n\nDecide:")

	return sb.String()
}

// Ask is a convenience for quick decisions without event context.
func (b *Brain) Ask(ctx context.Context, question string) (*Decision, error) {
	events := b.eventBus.History(20, "", "")
	return b.Decide(ctx, question, events)
}
