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

// Gemini implements the Provider interface for Google's Gemini API.
// Full format translation between OpenAI-compatible and Gemini native format.
type Gemini struct {
	config ProviderConfig
	client *http.Client
}

func NewGemini(cfg ProviderConfig) *Gemini {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &Gemini{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (g *Gemini) Name() string { return "gemini" }

// apiVersion returns "v1beta" for newer models, "v1" for stable ones.
func (g *Gemini) apiVersion(model string) string {
	// Gemini 2.x models require v1beta
	if strings.HasPrefix(model, "gemini-2") || strings.HasPrefix(model, "gemini-exp") {
		return "v1beta"
	}
	return "v1beta" // v1beta works for all models and has the latest features
}

func (g *Gemini) Send(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	body, err := g.buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}

	// URL: model name in path, auth via query param
	apiVer := g.apiVersion(req.Model)
	url := fmt.Sprintf("%s/%s/models/%s:generateContent?key=%s",
		g.config.BaseURL, apiVer, req.Model, g.config.APIKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, &ProviderAPIError{
			Provider:   "gemini",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	var gemResp geminiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&gemResp); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}

	return g.translateResponse(&gemResp, req.Model, time.Since(start)), nil
}

func (g *Gemini) SendStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	body, err := g.buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}

	apiVer := g.apiVersion(req.Model)
	url := fmt.Sprintf("%s/%s/models/%s:streamGenerateContent?alt=sse&key=%s",
		g.config.BaseURL, apiVer, req.Model, g.config.APIKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	streamClient := &http.Client{}
	httpResp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send stream request: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, &ProviderAPIError{
			Provider:   "gemini",
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		tokensSoFar := 0
		sentRole := false

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := line[6:]

			var gemChunk geminiResponse
			if err := json.Unmarshal([]byte(data), &gemChunk); err != nil {
				continue
			}

			// Send initial role delta
			if !sentRole {
				oaiChunk := `data: {"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`
				ch <- StreamChunk{Data: []byte(oaiChunk + "\n\n"), TokensSoFar: 0}
				sentRole = true
			}

			// Extract text from chunk
			text := ""
			if len(gemChunk.Candidates) > 0 && len(gemChunk.Candidates[0].Content.Parts) > 0 {
				text = gemChunk.Candidates[0].Content.Parts[0].Text
			}

			if text != "" {
				t := len(text) / 4
				if t == 0 {
					t = 1
				}
				tokensSoFar += t

				oaiChunk := fmt.Sprintf(`data: {"choices":[{"index":0,"delta":{"content":%s},"finish_reason":null}]}`,
					mustJSON(text))
				ch <- StreamChunk{Data: []byte(oaiChunk + "\n\n"), TokensSoFar: tokensSoFar}
			}

			// Check for finish reason (can arrive in the SAME chunk as text)
			if len(gemChunk.Candidates) > 0 && gemChunk.Candidates[0].FinishReason != "" {
				fr := mapGeminiFinishReason(gemChunk.Candidates[0].FinishReason)
				oaiChunk := fmt.Sprintf(`data: {"choices":[{"index":0,"delta":{},"finish_reason":"%s"}]}`, fr)
				ch <- StreamChunk{Data: []byte(oaiChunk + "\n\n"), TokensSoFar: tokensSoFar}
				ch <- StreamChunk{Data: []byte("data: [DONE]\n\n"), Done: true, TokensSoFar: tokensSoFar}
				return
			}
		}

		// Always close cleanly
		ch <- StreamChunk{Data: []byte("data: [DONE]\n\n"), Done: true, TokensSoFar: tokensSoFar}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("gemini: stream read: %w", err)}
		}
	}()

	return ch, nil
}

func (g *Gemini) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1beta/models?key=%s", g.config.BaseURL, g.config.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("gemini health check: invalid API key (status %d)", resp.StatusCode)
		}
		return fmt.Errorf("gemini health check: status %d", resp.StatusCode)
	}
	return nil
}

func (g *Gemini) buildRequestBody(req *Request) ([]byte, error) {
	// Translation: messages → contents, "assistant" → "model", system → systemInstruction
	var systemParts []string
	var contents []geminiContent
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	// Gemini requires at least one content entry
	if len(contents) == 0 {
		contents = append(contents, geminiContent{
			Role:  "user",
			Parts: []geminiPart{{Text: "(empty)"}},
		})
	}

	body := map[string]any{
		"contents": contents,
	}

	if len(systemParts) > 0 {
		body["systemInstruction"] = geminiContent{
			Parts: []geminiPart{{Text: strings.Join(systemParts, "\n\n")}},
		}
	}

	genConfig := map[string]any{}
	if req.Temperature != nil {
		genConfig["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		genConfig["maxOutputTokens"] = *req.MaxTokens
	}
	if len(genConfig) > 0 {
		body["generationConfig"] = genConfig
	}

	return json.Marshal(body)
}

func (g *Gemini) translateResponse(gemResp *geminiResponse, model string, latency time.Duration) *Response {
	resp := &Response{
		ID:       fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:   "chat.completion",
		Model:    model,
		Provider: "gemini",
		Latency:  latency,
	}

	if len(gemResp.Candidates) > 0 {
		c := gemResp.Candidates[0]
		var text string
		for _, p := range c.Content.Parts {
			text += p.Text
		}
		finishReason := mapGeminiFinishReason(c.FinishReason)
		resp.Choices = []Choice{{
			Index:        0,
			Message:      Message{Role: "assistant", Content: text},
			FinishReason: finishReason,
		}}
	}

	if gemResp.UsageMetadata != nil {
		resp.Usage = Usage{
			PromptTokens:     gemResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: gemResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gemResp.UsageMetadata.TotalTokenCount,
		}
	}

	return resp
}

func mapGeminiFinishReason(reason string) string {
	switch strings.ToUpper(reason) {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	default:
		return "stop"
	}
}

// Gemini types
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}
