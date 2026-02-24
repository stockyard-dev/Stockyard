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

// OpenAI implements the Provider interface for OpenAI's API.
// This is the canonical format — minimal translation needed.
type OpenAI struct {
	config ProviderConfig
	client *http.Client
}

// NewOpenAI creates a new OpenAI provider adapter.
func NewOpenAI(cfg ProviderConfig) *OpenAI {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &OpenAI{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) Send(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	body, err := buildOpenAIBody(req, false)
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		o.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, &ProviderAPIError{
			Provider:   "openai",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	resp := &Response{
		ID:       oaiResp.ID,
		Object:   oaiResp.Object,
		Model:    oaiResp.Model,
		Provider: "openai",
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

func (o *OpenAI) SendStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	req.Stream = true
	body, err := buildOpenAIBody(req, true)
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		o.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	// Use a separate client without timeout for streaming
	streamClient := &http.Client{}
	httpResp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send stream request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, &ProviderAPIError{
			Provider:   "openai",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()

		scanner := bufio.NewScanner(httpResp.Body)
		tokensSoFar := 0

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := line[6:]
			if data == "[DONE]" {
				ch <- StreamChunk{Data: []byte(line + "\n\n"), Done: true, TokensSoFar: tokensSoFar}
				return
			}

			// Count tokens from delta content
			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" {
						// Estimate: ~4 chars per token
						tokens := len(c.Delta.Content) / 4
						if tokens == 0 {
							tokens = 1
						}
						tokensSoFar += tokens
					}
				}
			}

			ch <- StreamChunk{
				Data:        []byte(line + "\n\n"),
				TokensSoFar: tokensSoFar,
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("openai: stream read: %w", err)}
		}
	}()

	return ch, nil
}

func (o *OpenAI) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", o.config.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.config.APIKey)
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai health check: status %d", resp.StatusCode)
	}
	return nil
}

// OpenAI response types (internal)
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// SendEmbedding sends an embedding request to OpenAI's /v1/embeddings endpoint.
func (o *OpenAI) SendEmbedding(ctx context.Context, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		o.config.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create embedding request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send embedding request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read embedding response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, &ProviderAPIError{
			Provider:   "openai",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	return respBody, nil
}
