// Package synth uses an LLM to synthesize answers from live system state.
// Given a question and collected data from all sources, it produces a
// coherent explanation that no single dashboard could show you.
package synth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/seance/collector"
)

// Answer holds the synthesized response.
type Answer struct {
	Question     string   `json:"question"`
	Response     string   `json:"response"`
	SourcesUsed  []string `json:"sources_used"`
	Confidence   string   `json:"confidence"` // high, medium, low
	LatencyMs    int      `json:"latency_ms"`
	TokensUsed   int      `json:"tokens_used"`
	CollectMs    int      `json:"collect_ms"`
	DataSources  int      `json:"data_sources"`
}

// Engine synthesizes answers from system state.
type Engine struct {
	llm   provider.Provider
	model string
}

// New creates a synthesis engine.
func New(llm provider.Provider, model string) *Engine {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Engine{llm: llm, model: model}
}

// Ask takes a question and system state, produces a synthesized answer.
func (e *Engine) Ask(ctx context.Context, question string, state *collector.SystemState) (*Answer, error) {
	start := time.Now()

	systemContext := collector.BuildContext(state)
	prompt := buildPrompt(question, systemContext)

	temp := 0.3
	maxTokens := 1500
	resp, err := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM synthesis failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	response := resp.Choices[0].Message.Content

	// Extract which sources were actually referenced
	sourcesUsed := findReferencedSources(response, state)

	// Determine confidence based on data quality
	confidence := assessConfidence(state, response)

	return &Answer{
		Question:    question,
		Response:    response,
		SourcesUsed: sourcesUsed,
		Confidence:  confidence,
		LatencyMs:   int(time.Since(start).Milliseconds()),
		TokensUsed:  resp.Usage.TotalTokens,
		CollectMs:   state.CollectMs,
		DataSources: state.SourceCount,
	}, nil
}

// FollowUp asks a follow-up question with previous context.
func (e *Engine) FollowUp(ctx context.Context, question string, state *collector.SystemState, previousQA []QAPair) (*Answer, error) {
	start := time.Now()

	systemContext := collector.BuildContext(state)
	prompt := buildFollowUpPrompt(question, systemContext, previousQA)

	temp := 0.3
	maxTokens := 1500
	resp, err := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
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

	return &Answer{
		Question:    question,
		Response:    resp.Choices[0].Message.Content,
		SourcesUsed: findReferencedSources(resp.Choices[0].Message.Content, state),
		Confidence:  assessConfidence(state, resp.Choices[0].Message.Content),
		LatencyMs:   int(time.Since(start).Milliseconds()),
		TokensUsed:  resp.Usage.TotalTokens,
		CollectMs:   state.CollectMs,
		DataSources: state.SourceCount,
	}, nil
}

// QAPair holds a previous question-answer for context.
type QAPair struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func buildPrompt(question, context string) string {
	var sb strings.Builder

	sb.WriteString("You are a production systems expert. You have direct access to live system state.\n")
	sb.WriteString("Answer the operator's question using ONLY the data provided below.\n\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Be specific: cite exact numbers, timestamps, error messages, and endpoint paths.\n")
	sb.WriteString("2. Explain causality: don't just say what's wrong, explain WHY and HOW it's connected.\n")
	sb.WriteString("3. Be honest: if the data doesn't contain enough information, say so clearly.\n")
	sb.WriteString("4. Suggest actions: end with 1-3 concrete things the operator could do right now.\n")
	sb.WriteString("5. Be concise: operators are busy. Lead with the answer, then explain.\n\n")

	sb.WriteString(context)
	sb.WriteString("\n\n")

	sb.WriteString("OPERATOR QUESTION: ")
	sb.WriteString(question)
	sb.WriteString("\n\nAnswer:")

	return sb.String()
}

func buildFollowUpPrompt(question, context string, history []QAPair) string {
	var sb strings.Builder

	sb.WriteString("You are a production systems expert in an ongoing diagnostic conversation.\n")
	sb.WriteString("Use the live system data AND the conversation history to answer.\n\n")

	sb.WriteString(context)
	sb.WriteString("\n\n")

	if len(history) > 0 {
		sb.WriteString("CONVERSATION HISTORY:\n")
		for _, qa := range history {
			sb.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", qa.Question, truncate(qa.Answer, 500)))
		}
	}

	sb.WriteString("FOLLOW-UP QUESTION: ")
	sb.WriteString(question)
	sb.WriteString("\n\nAnswer:")

	return sb.String()
}

func findReferencedSources(response string, state *collector.SystemState) []string {
	var sources []string
	responseLower := strings.ToLower(response)
	for _, snap := range state.Snapshots {
		if strings.Contains(responseLower, strings.ToLower(snap.Source)) {
			sources = append(sources, snap.Source)
		}
	}
	if len(sources) == 0 {
		// If no explicit references, list all non-error sources
		for _, snap := range state.Snapshots {
			if snap.Error == "" {
				sources = append(sources, snap.Source)
			}
		}
	}
	return sources
}

func assessConfidence(state *collector.SystemState, response string) string {
	// High confidence: multiple sources, no errors, substantial data
	if state.ErrorCount == 0 && state.SourceCount >= 3 {
		return "high"
	}
	// Low confidence: many errors or very few sources
	if state.ErrorCount > state.SourceCount/2 || state.SourceCount <= 1 {
		return "low"
	}
	return "medium"
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// FormatAnswer produces a pretty-printed answer for terminal display.
func FormatAnswer(a *Answer) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(a.Response)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  Sources: %s\n", strings.Join(a.SourcesUsed, ", ")))
	sb.WriteString(fmt.Sprintf("  Confidence: %s\n", a.Confidence))
	sb.WriteString(fmt.Sprintf("  Data: %d sources queried in %dms, answer in %dms\n",
		a.DataSources, a.CollectMs, a.LatencyMs))

	return sb.String()
}

// Summarize produces a brief system status without a specific question.
func (e *Engine) Summarize(ctx context.Context, state *collector.SystemState) (*Answer, error) {
	return e.Ask(ctx, "Give me a brief status summary of this system. What's healthy, what's degraded, anything unusual?", state)
}

// MarshalHistory serializes Q&A history to JSON.
func MarshalHistory(pairs []QAPair) string {
	data, marshalErr := json.Marshal(pairs)
	if marshalErr != nil {
		data = []byte("{}")
	}
	return string(data)
}
