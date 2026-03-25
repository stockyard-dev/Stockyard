// Package stream provides streaming synthesis for Seance, delivering answer
// tokens incrementally via SSE or channel-based interfaces.
package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/synth"
)

// Token represents a single chunk of streaming output.
type Token struct {
	// Text is the token content.
	Text string `json:"text"`
	// Done indicates this is the final token.
	Done bool `json:"done"`
	// Error, if non-empty, indicates a stream error.
	Error string `json:"error,omitempty"`
	// Metadata is sent only with the final token.
	Metadata *TokenMetadata `json:"metadata,omitempty"`
}

// TokenMetadata holds answer metadata sent with the final streaming token.
type TokenMetadata struct {
	SourcesUsed []string `json:"sources_used"`
	Confidence  string   `json:"confidence"`
	LatencyMs   int      `json:"latency_ms"`
	CollectMs   int      `json:"collect_ms"`
	DataSources int      `json:"data_sources"`
	TokensUsed  int      `json:"tokens_used"`
}

// StreamSynth wraps a provider to deliver streaming answers.
type StreamSynth struct {
	llm   provider.Provider
	model string
}

// New creates a StreamSynth.
func New(llm provider.Provider, model string) *StreamSynth {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &StreamSynth{llm: llm, model: model}
}

// AskStream collects system state and streams the synthesized answer token by token.
// The returned channel will be closed when the stream ends.
func (ss *StreamSynth) AskStream(ctx context.Context, question string, state *collector.SystemState, history []synth.QAPair) (<-chan Token, error) {
	systemContext := collector.BuildContext(state)
	prompt := buildStreamPrompt(question, systemContext, history)

	temp := 0.3
	maxTokens := 1500
	req := &provider.Request{
		Model:       ss.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      true,
	}

	out := make(chan Token, 64)

	// Try streaming from the provider
	streamCh, err := ss.llm.SendStream(ctx, req)
	if err != nil {
		// Fall back to non-streaming: send full response then simulate streaming
		go ss.fallbackStream(ctx, req, state, out)
		return out, nil
	}

	go ss.processStream(ctx, streamCh, state, out)
	return out, nil
}

// processStream reads from the provider's stream channel and emits tokens.
func (ss *StreamSynth) processStream(ctx context.Context, streamCh <-chan provider.StreamChunk, state *collector.SystemState, out chan<- Token) {
	defer close(out)

	start := time.Now()
	var fullText strings.Builder
	totalTokens := 0

	for chunk := range streamCh {
		if chunk.Error != nil {
			out <- Token{Error: chunk.Error.Error(), Done: true}
			return
		}
		if chunk.Done {
			break
		}

		totalTokens = chunk.TokensSoFar

		// Parse SSE data to extract text content
		text := extractTextFromSSE(chunk.Data)
		if text != "" {
			fullText.WriteString(text)
			out <- Token{Text: text}
		}
	}

	// Send final token with metadata
	response := fullText.String()
	sourcesUsed := findReferencedSources(response, state)
	confidence := assessConfidence(state)

	out <- Token{
		Done: true,
		Metadata: &TokenMetadata{
			SourcesUsed: sourcesUsed,
			Confidence:  confidence,
			LatencyMs:   int(time.Since(start).Milliseconds()),
			CollectMs:   state.CollectMs,
			DataSources: state.SourceCount,
			TokensUsed:  totalTokens,
		},
	}
}

// fallbackStream sends a non-streaming request, then simulates streaming by
// emitting the response in small chunks.
func (ss *StreamSynth) fallbackStream(ctx context.Context, req *provider.Request, state *collector.SystemState, out chan<- Token) {
	defer close(out)

	start := time.Now()
	req.Stream = false

	resp, err := ss.llm.Send(ctx, req)
	if err != nil {
		out <- Token{Error: err.Error(), Done: true}
		return
	}

	if len(resp.Choices) == 0 {
		out <- Token{Error: "no response from LLM", Done: true}
		return
	}

	response := resp.Choices[0].Message.Content

	// Simulate streaming by chunking the response
	chunkSize := 4
	for i := 0; i < len(response); i += chunkSize {
		end := i + chunkSize
		if end > len(response) {
			end = len(response)
		}
		out <- Token{Text: response[i:end]}

		// Small delay for streaming feel
		select {
		case <-ctx.Done():
			out <- Token{Error: "cancelled", Done: true}
			return
		case <-time.After(10 * time.Millisecond):
		}
	}

	sourcesUsed := findReferencedSources(response, state)
	confidence := assessConfidence(state)

	out <- Token{
		Done: true,
		Metadata: &TokenMetadata{
			SourcesUsed: sourcesUsed,
			Confidence:  confidence,
			LatencyMs:   int(time.Since(start).Milliseconds()),
			CollectMs:   state.CollectMs,
			DataSources: state.SourceCount,
			TokensUsed:  resp.Usage.TotalTokens,
		},
	}
}

// extractTextFromSSE parses SSE data lines to extract text content.
func extractTextFromSSE(data []byte) string {
	s := string(data)
	// Handle OpenAI-style SSE format: data: {"choices":[{"delta":{"content":"..."}}]}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "data: ")
		if jsonStr == "[DONE]" {
			return ""
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			// Not JSON, treat raw data as text
			return jsonStr
		}
		if len(chunk.Choices) > 0 {
			return chunk.Choices[0].Delta.Content
		}
	}

	// If no SSE prefix, try raw content
	if !strings.Contains(s, "data:") && len(s) > 0 {
		return s
	}
	return ""
}

func buildStreamPrompt(question, sysContext string, history []synth.QAPair) string {
	var sb strings.Builder

	sb.WriteString("You are a production systems expert. You have direct access to live system state.\n")
	sb.WriteString("Answer the operator's question using ONLY the data provided below.\n\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Be specific: cite exact numbers, timestamps, error messages, and endpoint paths.\n")
	sb.WriteString("2. Explain causality: don't just say what's wrong, explain WHY and HOW it's connected.\n")
	sb.WriteString("3. Be honest: if the data doesn't contain enough information, say so clearly.\n")
	sb.WriteString("4. Suggest actions: end with 1-3 concrete things the operator could do right now.\n")
	sb.WriteString("5. Be concise: operators are busy. Lead with the answer, then explain.\n\n")

	sb.WriteString(sysContext)
	sb.WriteString("\n\n")

	if len(history) > 0 {
		sb.WriteString("CONVERSATION HISTORY:\n")
		for _, qa := range history {
			sb.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", qa.Question, truncate(qa.Answer, 500)))
		}
	}

	sb.WriteString("OPERATOR QUESTION: ")
	sb.WriteString(question)
	sb.WriteString("\n\nAnswer:")

	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func findReferencedSources(response string, state *collector.SystemState) []string {
	var sources []string
	lower := strings.ToLower(response)
	for _, snap := range state.Snapshots {
		if strings.Contains(lower, strings.ToLower(snap.Source)) {
			sources = append(sources, snap.Source)
		}
	}
	if len(sources) == 0 {
		for _, snap := range state.Snapshots {
			if snap.Error == "" {
				sources = append(sources, snap.Source)
			}
		}
	}
	return sources
}

func assessConfidence(state *collector.SystemState) string {
	if state.ErrorCount == 0 && state.SourceCount >= 3 {
		return "high"
	}
	if state.ErrorCount > state.SourceCount/2 || state.SourceCount <= 1 {
		return "low"
	}
	return "medium"
}
