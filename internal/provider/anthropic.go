package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Anthropic implements the Provider interface for Anthropic's Messages API.
// Translates between OpenAI-compatible format and Anthropic's native format.
type Anthropic struct {
	config ProviderConfig
	client *http.Client
}

func NewAnthropic(cfg ProviderConfig) *Anthropic {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &Anthropic{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Send(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	body, err := a.buildRequestBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.config.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	a.setHeaders(httpReq)

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, &ProviderAPIError{
			Provider:   "anthropic",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	var antResp anthropicResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&antResp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	return a.translateResponse(&antResp, time.Since(start)), nil
}

func (a *Anthropic) SendStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	body, err := a.buildRequestBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.config.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	a.setHeaders(httpReq)

	streamClient := &http.Client{}
	httpResp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send stream request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, &ProviderAPIError{
			Provider:   "anthropic",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()

		scanner := bufio.NewScanner(httpResp.Body)
		// Increase buffer for long responses
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		tokensSoFar := 0
		sentRole := false

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and event: lines (Anthropic SSE format uses
			// "event: <type>\ndata: <json>" pairs — we only need the data lines)
			if line == "" || strings.HasPrefix(line, "event:") {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := line[6:]

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "message_start":
				// Send the initial role delta (OpenAI sends role in first chunk)
				if !sentRole {
					oaiChunk := `data: {"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`
					ch <- StreamChunk{Data: []byte(oaiChunk + "\n\n"), TokensSoFar: 0}
					sentRole = true
				}

			case "content_block_start":
				// Anthropic sends this before content_block_delta; we handle text in deltas.
				// Nothing to translate here for text blocks.
				continue

			case "content_block_delta":
				text := event.Delta.Text
				if text == "" {
					continue
				}
				tokens := len(text) / 4
				if tokens == 0 {
					tokens = 1
				}
				tokensSoFar += tokens

				// Translate to OpenAI SSE format
				oaiChunk := fmt.Sprintf(`data: {"choices":[{"index":0,"delta":{"content":%s},"finish_reason":null}]}`,
					mustJSON(text))
				ch <- StreamChunk{
					Data:        []byte(oaiChunk + "\n\n"),
					TokensSoFar: tokensSoFar,
				}

			case "content_block_stop":
				// End of a content block; no OpenAI equivalent needed.
				continue

			case "message_delta":
				// Final message metadata — contains stop_reason and usage
				fr := mapAnthropicFinishReason(event.Delta.StopReason)
				oaiChunk := fmt.Sprintf(`data: {"choices":[{"index":0,"delta":{},"finish_reason":"%s"}]}`, fr)
				ch <- StreamChunk{
					Data:        []byte(oaiChunk + "\n\n"),
					TokensSoFar: tokensSoFar,
				}

			case "message_stop":
				ch <- StreamChunk{
					Data:        []byte("data: [DONE]\n\n"),
					Done:        true,
					TokensSoFar: tokensSoFar,
				}
				return

			case "ping":
				// Keep-alive from Anthropic; ignore silently.
				continue

			case "error":
				// Anthropic stream error event
				errMsg := event.Error.Message
				if errMsg == "" {
					errMsg = "unknown stream error"
				}
				ch <- StreamChunk{Error: fmt.Errorf("anthropic: stream error: %s", errMsg)}
				return
			}
		}

		// If we reach here without message_stop, still send DONE so the client isn't left hanging
		ch <- StreamChunk{Data: []byte("data: [DONE]\n\n"), Done: true, TokensSoFar: tokensSoFar}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("anthropic: stream read: %w", err)}
		}
	}()

	return ch, nil
}

func (a *Anthropic) HealthCheck(ctx context.Context) error {
	// Anthropic doesn't have a lightweight health endpoint.
	// Send a minimal invalid request — 400 means the API is up.
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		a.config.BaseURL+"/v1/messages", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return err
	}
	a.setHeaders(httpReq)
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	// 400 (bad request) or 422 means the API is up, our request was just invalid
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == 422 || resp.StatusCode == http.StatusOK {
		return nil
	}
	// 401 means bad API key but service is up
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("anthropic health check: invalid API key (status 401)")
	}
	return fmt.Errorf("anthropic health check: status %d", resp.StatusCode)
}

func (a *Anthropic) setHeaders(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("x-api-key", a.config.APIKey)
	r.Header.Set("anthropic-version", "2024-10-22")
}

func (a *Anthropic) buildRequestBody(req *Request, stream bool) ([]byte, error) {
	// Translation: Extract system messages from messages array → top-level "system" param.
	// Anthropic supports multiple system messages concatenated.
	var systemParts []string
	var messages []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
		} else {
			messages = append(messages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	// Anthropic requires at least one message
	if len(messages) == 0 {
		messages = append(messages, anthropicMessage{
			Role:    "user",
			Content: "(empty)",
		})
	}

	body := map[string]any{
		"model":    req.Model,
		"messages": messages,
	}

	// max_tokens is required for Anthropic, default 4096
	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	body["max_tokens"] = maxTokens

	if len(systemParts) > 0 {
		body["system"] = strings.Join(systemParts, "\n\n")
	}
	if stream {
		body["stream"] = true
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}

	return json.Marshal(body)
}

func (a *Anthropic) translateResponse(antResp *anthropicResponse, latency time.Duration) *Response {
	resp := &Response{
		ID:       antResp.ID,
		Object:   "chat.completion",
		Model:    antResp.Model,
		Provider: "anthropic",
		Latency:  latency,
		Usage: Usage{
			PromptTokens:     antResp.Usage.InputTokens,
			CompletionTokens: antResp.Usage.OutputTokens,
			TotalTokens:      antResp.Usage.InputTokens + antResp.Usage.OutputTokens,
		},
	}

	// Concatenate all text content blocks
	var content string
	for _, block := range antResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	finishReason := mapAnthropicFinishReason(antResp.StopReason)

	resp.Choices = []Choice{{
		Index:        0,
		Message:      Message{Role: "assistant", Content: content},
		FinishReason: finishReason,
	}}

	return resp
}

func mapAnthropicFinishReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

// mustJSON marshals a value and returns the JSON string.
func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// anthropicMessage is the Anthropic-format message for request bodies.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Anthropic response types
type anthropicResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
