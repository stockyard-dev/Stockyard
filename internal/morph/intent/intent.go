// Package intent resolves natural language intents into structured API operations.
// This is the core intelligence layer: given "charge a customer $50", it produces
// a structured operation spec that the executor can translate to real API calls.
package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Operation is a structured representation of what the user wants to do.
type Operation struct {
	Service    string         `json:"service"`     // stripe, github, slack, etc.
	Action     string         `json:"action"`      // charge, create_pr, send_message, etc.
	Parameters map[string]any `json:"parameters"`  // action-specific params
	Confidence float64        `json:"confidence"`  // 0-1 how confident we are in the parse
	Reasoning  string         `json:"reasoning"`   // why we interpreted it this way
}

// MultiOperation represents a workflow spanning multiple services.
type MultiOperation struct {
	Steps     []Operation `json:"steps"`
	Trigger   string      `json:"trigger,omitempty"`   // event that starts the workflow
	Condition string      `json:"condition,omitempty"` // when to execute
}

// Resolver parses natural language intents into operations.
type Resolver struct {
	llm   provider.Provider
	model string
}

// New creates an intent resolver.
func New(llm provider.Provider, model string) *Resolver {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Resolver{llm: llm, model: model}
}

// Resolve parses a natural language intent into structured operations.
func (r *Resolver) Resolve(ctx context.Context, intent string, service string, availableServices []string) (*Operation, error) {
	prompt := buildResolvePrompt(intent, service, availableServices)

	temp := 0.1
	maxTokens := 500
	resp, err := r.llm.Send(ctx, &provider.Request{
		Model:       r.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM resolution failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var op Operation
	if err := json.Unmarshal([]byte(raw), &op); err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w (raw: %s)", err, truncate(raw, 200))
	}

	return &op, nil
}

// ResolveMulti parses a multi-service intent into a workflow.
func (r *Resolver) ResolveMulti(ctx context.Context, intent string, availableServices []string) (*MultiOperation, error) {
	prompt := buildMultiResolvePrompt(intent, availableServices)

	temp := 0.1
	maxTokens := 1000
	resp, err := r.llm.Send(ctx, &provider.Request{
		Model:       r.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var multi MultiOperation
	if err := json.Unmarshal([]byte(raw), &multi); err != nil {
		return nil, fmt.Errorf("parsing multi-operation: %w", err)
	}
	return &multi, nil
}

func buildResolvePrompt(intent, service string, available []string) string {
	var sb strings.Builder
	sb.WriteString("You are an API intent resolver. Given a natural language request, produce a structured operation.\n\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Respond with ONLY JSON, no other text.\n")
	sb.WriteString("2. Identify the service, action, and parameters.\n")
	sb.WriteString("3. Use standard parameter names (amount in cents for payments, etc.).\n")
	sb.WriteString("4. Set confidence 0-1 based on how clear the intent is.\n\n")

	if service != "" {
		sb.WriteString(fmt.Sprintf("TARGET SERVICE: %s\n\n", service))
	}
	if len(available) > 0 {
		sb.WriteString(fmt.Sprintf("AVAILABLE SERVICES: %s\n\n", strings.Join(available, ", ")))
	}

	sb.WriteString(fmt.Sprintf("USER INTENT: %s\n\n", intent))
	sb.WriteString("Respond with JSON:\n")
	sb.WriteString(`{"service":"...","action":"...","parameters":{...},"confidence":0.0-1.0,"reasoning":"..."}`)
	return sb.String()
}

func buildMultiResolvePrompt(intent string, available []string) string {
	var sb strings.Builder
	sb.WriteString("You are an API workflow resolver. Given a natural language request that may span multiple services, produce a sequence of operations.\n\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Respond with ONLY JSON.\n")
	sb.WriteString("2. Each step has: service, action, parameters.\n")
	sb.WriteString("3. Steps execute in order. Later steps can reference results of earlier steps.\n")
	sb.WriteString("4. Use {{step_N.field}} to reference previous step results.\n\n")

	if len(available) > 0 {
		sb.WriteString(fmt.Sprintf("AVAILABLE SERVICES: %s\n\n", strings.Join(available, ", ")))
	}

	sb.WriteString(fmt.Sprintf("USER INTENT: %s\n\n", intent))
	sb.WriteString("Respond with JSON:\n")
	sb.WriteString(`{"steps":[{"service":"...","action":"...","parameters":{...},"confidence":0.9,"reasoning":"..."}],"trigger":"...","condition":"..."}`)
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
