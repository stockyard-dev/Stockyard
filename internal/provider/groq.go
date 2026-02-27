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

// Groq implements the Provider interface for Groq's API.
// Groq is OpenAI-compatible — same format, different base URL + models.
type Groq struct {
	config ProviderConfig
	client *http.Client
}

func NewGroq(cfg ProviderConfig) *Groq {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.groq.com/openai/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Groq{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (g *Groq) Name() string { return "groq" }

func (g *Groq) Send(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	body, err := buildOpenAIBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("groq: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		g.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("groq: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.config.APIKey)

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("groq: send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, &ProviderAPIError{
			Provider:   "groq",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("groq: decode response: %w", err)
	}

	resp := &Response{
		ID:       oaiResp.ID,
		Object:   oaiResp.Object,
		Model:    oaiResp.Model,
		Provider: "groq",
		Latency:  time.Since(start),
		Usage: Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		},
	}
	for _, c := range oaiResp.Choices {
		resp.Choices = append(resp.Choices, Choice{
			Index:        c.Index,
			Message:      Message{Role: c.Message.Role, Content: c.Message.Content},
			FinishReason: c.FinishReason,
		})
	}
	return resp, nil
}

func (g *Groq) SendStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	body, err := buildOpenAIBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("groq: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		g.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("groq: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.config.APIKey)

	streamClient := &http.Client{}
	httpResp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("groq: send stream request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, &ProviderAPIError{
			Provider:   "groq",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		tokensSoFar := 0

		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := line[6:]
			if data == "[DONE]" {
				ch <- StreamChunk{Data: []byte(line + "\n\n"), Done: true, TokensSoFar: tokensSoFar}
				return
			}
			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" {
						t := len(c.Delta.Content) / 4
						if t == 0 { t = 1 }
						tokensSoFar += t
					}
				}
			}
			ch <- StreamChunk{Data: []byte(line + "\n\n"), TokensSoFar: tokensSoFar}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("groq: stream read: %w", err)}
		}
	}()
	return ch, nil
}

func (g *Groq) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", g.config.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.config.APIKey)
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("groq health check: status %d", resp.StatusCode)
	}
	return nil
}

// buildOpenAIBody builds a standard OpenAI-format request body.
// Shared by OpenAI and Groq adapters.
func buildOpenAIBody(req *Request, stream bool) ([]byte, error) {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}
	if stream {
		body["stream"] = true
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	// Only forward known OpenAI parameters from Extra.
	// Everything else (internal fields, transport headers, middleware state) is dropped.
	allowedExtra := map[string]bool{
		"tools": true, "tool_choice": true, "response_format": true,
		"top_p": true, "frequency_penalty": true, "presence_penalty": true,
		"stop": true, "n": true, "logprobs": true, "top_logprobs": true,
		"seed": true, "user": true, "logit_bias": true,
		"parallel_tool_calls": true, "service_tier": true,
		"store": true, "metadata": true, "stream_options": true,
		"reasoning_effort": true, "max_completion_tokens": true,
	}
	for k, v := range req.Extra {
		if allowedExtra[k] {
			body[k] = v
		}
	}
	return json.Marshal(body)
}
