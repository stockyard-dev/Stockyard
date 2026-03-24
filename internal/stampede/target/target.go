// Package target provides HTTP clients for communicating with target AI applications.
package target

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Format specifies the target API format.
type Format string

const (
	FormatAuto   Format = "auto"
	FormatOpenAI Format = "openai"
	FormatSimple Format = "simple"
)

// Config holds target client configuration.
type Config struct {
	URL      string
	Format   Format
	ProxyURL string
	Timeout  time.Duration
}

// RequestMeta holds optional metadata for tracking through a proxy.
type RequestMeta struct {
	SimID          string
	ConversationID string
}

// Client sends messages to a target AI application.
type Client struct {
	cfg        Config
	httpClient *http.Client
	detected   Format
	meta       RequestMeta
}

// New creates a new target client.
func New(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// URL returns the target URL.
func (c *Client) URL() string { return c.cfg.URL }

// SetMeta sets tracking metadata for proxy enrichment.
func (c *Client) SetMeta(m RequestMeta) { c.meta = m }

// Send sends a user message to the target and returns the response text.
func (c *Client) Send(ctx context.Context, message string) (string, error) {
	format := c.cfg.Format
	if format == FormatAuto {
		if c.detected != "" {
			format = c.detected
		} else {
			format = c.detectFormat()
		}
	}

	switch format {
	case FormatOpenAI:
		return c.sendOpenAI(ctx, message)
	case FormatSimple:
		return c.sendSimple(ctx, message)
	default:
		// Try OpenAI first, fall back to simple
		resp, err := c.sendOpenAI(ctx, message)
		if err == nil {
			c.detected = FormatOpenAI
			return resp, nil
		}
		resp, err = c.sendSimple(ctx, message)
		if err == nil {
			c.detected = FormatSimple
			return resp, nil
		}
		return "", fmt.Errorf("could not communicate with target: %v", err)
	}
}

// sendOpenAI sends a request in OpenAI chat completions format.
func (c *Client) sendOpenAI(ctx context.Context, message string) (string, error) {
	body := map[string]any{
		"messages": []map[string]string{
			{"role": "user", "content": message},
		},
		"model": "default",
	}
	data, _ := json.Marshal(body)

	url := c.cfg.URL
	if !strings.HasSuffix(url, "/chat/completions") && !strings.HasSuffix(url, "/v1/chat/completions") {
		url = strings.TrimSuffix(url, "/") + "/v1/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addStampedeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("target returned %d: %s", resp.StatusCode, truncBody(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing OpenAI response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

// sendSimple sends a request in simple JSON format: {"message": "..."} -> {"response": "..."}
func (c *Client) sendSimple(ctx context.Context, message string) (string, error) {
	body := map[string]string{"message": message}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addStampedeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("target returned %d: %s", resp.StatusCode, truncBody(respBody))
	}

	// Try multiple response field names
	var generic map[string]any
	if err := json.Unmarshal(respBody, &generic); err != nil {
		// Not JSON — treat entire body as text response
		return strings.TrimSpace(string(respBody)), nil
	}

	for _, key := range []string{"response", "reply", "message", "content", "text", "answer", "output"} {
		if v, ok := generic[key]; ok {
			if s, ok := v.(string); ok {
				return s, nil
			}
		}
	}

	return string(respBody), nil
}

// detectFormat probes the target URL to determine the API format.
func (c *Client) detectFormat() Format {
	if strings.Contains(c.cfg.URL, "/v1/") || strings.Contains(c.cfg.URL, "/chat/completions") {
		return FormatOpenAI
	}
	return FormatSimple
}

func truncBody(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// addStampedeHeaders adds tracking headers for proxy enrichment.
func (c *Client) addStampedeHeaders(req *http.Request) {
	req.Header.Set("X-Stampede", "true")
	if c.meta.SimID != "" {
		req.Header.Set("X-Stampede-Sim", c.meta.SimID)
		req.Header.Set("X-Stockyard-Tags", "stampede_sim:"+c.meta.SimID)
	}
	if c.meta.ConversationID != "" {
		req.Header.Set("X-Stampede-Conversation", c.meta.ConversationID)
		if c.meta.SimID != "" {
			req.Header.Set("X-Stockyard-Tags", "stampede_sim:"+c.meta.SimID+",stampede_conversation:"+c.meta.ConversationID)
		}
	}
}
