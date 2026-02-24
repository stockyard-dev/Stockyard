package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================
// OpenAI adapter tests
// ============================================================

func TestOpenAI_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-4o" {
			t.Errorf("model = %v", body["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-123",
			"object": "chat.completion",
			"model":  "gpt-4o",
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": "Hello from OpenAI!"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	oai := NewOpenAI(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
	})

	resp, err := oai.Send(context.Background(), &Request{
		Model:    "gpt-4o",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if resp.Provider != "openai" {
		t.Errorf("provider = %q", resp.Provider)
	}
	if resp.Choices[0].Message.Content != "Hello from OpenAI!" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("total tokens = %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAI_SendStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	oai := NewOpenAI(ProviderConfig{APIKey: "test-key", BaseURL: server.URL + "/v1"})
	ch, err := oai.SendStream(context.Background(), &Request{
		Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}

	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Fatal("no chunks received")
	}
	// Last chunk should be DONE
	if !chunks[len(chunks)-1].Done {
		t.Error("last chunk should be Done")
	}
}

func TestOpenAI_Error_Returns_ProviderAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited","type":"rate_limit_error"}}`)
	}))
	defer server.Close()

	oai := NewOpenAI(ProviderConfig{APIKey: "test-key", BaseURL: server.URL + "/v1"})
	_, err := oai.Send(context.Background(), &Request{
		Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*ProviderAPIError)
	if !ok {
		t.Fatalf("expected ProviderAPIError, got %T", err)
	}
	if apiErr.StatusCode != 429 {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if !apiErr.IsRetryable() {
		t.Error("429 should be retryable")
	}
}

// ============================================================
// Anthropic adapter tests
// ============================================================

func TestAnthropic_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Anthropic-specific headers
		if r.Header.Get("x-api-key") != "ant-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2024-10-22" {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}

		// Verify request translation
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// System message should be extracted to top-level "system" field
		if body["system"] != "You are helpful." {
			t.Errorf("system = %v", body["system"])
		}

		// Messages should NOT contain system message
		msgs := body["messages"].([]any)
		for _, m := range msgs {
			msg := m.(map[string]any)
			if msg["role"] == "system" {
				t.Error("system message should be extracted from messages array")
			}
		}

		// max_tokens should be present (required by Anthropic)
		if body["max_tokens"] == nil {
			t.Error("max_tokens should be set (required by Anthropic)")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_123",
			"type":        "message",
			"model":       "claude-sonnet-4-5-20250929",
			"stop_reason": "end_turn",
			"content": []map[string]string{
				{"type": "text", "text": "Hello from Anthropic!"},
			},
			"usage": map[string]int{
				"input_tokens":  12,
				"output_tokens": 8,
			},
		})
	}))
	defer server.Close()

	ant := NewAnthropic(ProviderConfig{APIKey: "ant-key", BaseURL: server.URL})
	resp, err := ant.Send(context.Background(), &Request{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	// Verify response translation to OpenAI format
	if resp.Provider != "anthropic" {
		t.Errorf("provider = %q", resp.Provider)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %q", resp.Object)
	}
	if resp.Choices[0].Message.Content != "Hello from Anthropic!" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("role = %q", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %q (should map end_turn → stop)", resp.Choices[0].FinishReason)
	}
	if resp.Usage.PromptTokens != 12 {
		t.Errorf("prompt_tokens = %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 8 {
		t.Errorf("completion_tokens = %d", resp.Usage.CompletionTokens)
	}
}

func TestAnthropic_SendStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")

		// Anthropic SSE format: event: type\ndata: json
		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\"}}",
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0}",
			"event: ping\ndata: {\"type\":\"ping\"}",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}",
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\"}",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}",
		}
		for _, e := range events {
			fmt.Fprintf(w, "%s\n\n", e)
			flusher.Flush()
		}
	}))
	defer server.Close()

	ant := NewAnthropic(ProviderConfig{APIKey: "ant-key", BaseURL: server.URL})
	ch, err := ant.SendStream(context.Background(), &Request{
		Model: "claude-sonnet-4-5-20250929", Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}

	var assembled string
	var gotDone bool
	var gotRole bool
	var gotFinish bool

	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		data := string(chunk.Data)

		// Check for role delta
		if strings.Contains(data, `"role":"assistant"`) {
			gotRole = true
		}

		// Extract content from translated OpenAI chunks
		if strings.Contains(data, `"content":`) && !strings.Contains(data, `"content":""`) {
			var oaiChunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			// Strip "data: " prefix
			jsonStr := strings.TrimPrefix(strings.TrimSpace(data), "data: ")
			if err := json.Unmarshal([]byte(jsonStr), &oaiChunk); err == nil && len(oaiChunk.Choices) > 0 {
				assembled += oaiChunk.Choices[0].Delta.Content
			}
		}

		if strings.Contains(data, `"finish_reason":"stop"`) {
			gotFinish = true
		}

		if chunk.Done {
			gotDone = true
		}
	}

	if !gotRole {
		t.Error("should have received role delta")
	}
	if assembled != "Hello world" {
		t.Errorf("assembled = %q, want %q", assembled, "Hello world")
	}
	if !gotFinish {
		t.Error("should have received finish_reason")
	}
	if !gotDone {
		t.Error("should have received DONE")
	}
}

func TestAnthropic_MultipleSystemMessages(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_1", "type": "message", "model": "claude-sonnet-4-5-20250929",
			"stop_reason": "end_turn",
			"content":     []map[string]string{{"type": "text", "text": "ok"}},
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	ant := NewAnthropic(ProviderConfig{APIKey: "key", BaseURL: server.URL})
	ant.Send(context.Background(), &Request{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []Message{
			{Role: "system", Content: "First system msg."},
			{Role: "system", Content: "Second system msg."},
			{Role: "user", Content: "Hi"},
		},
	})

	// Multiple system messages should be concatenated
	sys := capturedBody["system"].(string)
	if !strings.Contains(sys, "First system msg.") || !strings.Contains(sys, "Second system msg.") {
		t.Errorf("system = %q, want both system messages concatenated", sys)
	}
}

// ============================================================
// Groq adapter tests
// ============================================================

func TestGroq_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Groq uses Bearer auth like OpenAI
		if r.Header.Get("Authorization") != "Bearer groq-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "groq-123", "object": "chat.completion", "model": "llama-3.3-70b-versatile",
			"choices": []map[string]any{{
				"index": 0, "message": map[string]string{"role": "assistant", "content": "Hello from Groq!"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12},
		})
	}))
	defer server.Close()

	g := NewGroq(ProviderConfig{APIKey: "groq-key", BaseURL: server.URL + "/openai/v1"})
	resp, err := g.Send(context.Background(), &Request{
		Model: "llama-3.3-70b-versatile", Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if resp.Provider != "groq" {
		t.Errorf("provider = %q", resp.Provider)
	}
	if resp.Choices[0].Message.Content != "Hello from Groq!" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
}

// ============================================================
// Gemini adapter tests
// ============================================================

func TestGemini_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL format: /v1beta/models/MODEL:generateContent?key=KEY
		if !strings.Contains(r.URL.Path, "models/gemini-2.0-flash") {
			t.Errorf("path = %q", r.URL.Path)
		}
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Errorf("path should contain :generateContent, got %q", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "gem-key" {
			t.Errorf("key = %q", r.URL.Query().Get("key"))
		}

		// Verify request translation
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// Messages should be "contents" with "model" role instead of "assistant"
		contents := body["contents"].([]any)
		if len(contents) == 0 {
			t.Fatal("no contents")
		}

		// System should be systemInstruction
		if body["systemInstruction"] == nil {
			t.Error("system message should be in systemInstruction")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"parts": []map[string]string{{"text": "Hello from Gemini!"}},
					"role":  "model",
				},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]int{
				"promptTokenCount":     15,
				"candidatesTokenCount": 6,
				"totalTokenCount":      21,
			},
		})
	}))
	defer server.Close()

	gem := NewGemini(ProviderConfig{APIKey: "gem-key", BaseURL: server.URL})
	resp, err := gem.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "system", Content: "Be concise."},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if resp.Provider != "gemini" {
		t.Errorf("provider = %q", resp.Provider)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %q", resp.Object)
	}
	if resp.Choices[0].Message.Content != "Hello from Gemini!" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("role = %q (Gemini 'model' should map to 'assistant')", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %q (STOP should map to stop)", resp.Choices[0].FinishReason)
	}
	if resp.Usage.PromptTokens != 15 {
		t.Errorf("prompt_tokens = %d", resp.Usage.PromptTokens)
	}
}

func TestGemini_SendStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":streamGenerateContent") {
			t.Errorf("stream path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("alt") != "sse" {
			t.Errorf("alt = %q, want sse", r.URL.Query().Get("alt"))
		}

		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")

		chunks := []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":" Gemini"}],"role":"model"}}]}`,
			// Final chunk with text AND finishReason in same message
			`data: {"candidates":[{"content":{"parts":[{"text":"!"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer server.Close()

	gem := NewGemini(ProviderConfig{APIKey: "gem-key", BaseURL: server.URL})
	ch, err := gem.SendStream(context.Background(), &Request{
		Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}

	var assembled string
	var gotDone bool
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		data := string(chunk.Data)

		// Extract content
		if strings.Contains(data, `"content":`) && !strings.Contains(data, `"content":""`) {
			jsonStr := strings.TrimPrefix(strings.TrimSpace(data), "data: ")
			var oaiChunk struct {
				Choices []struct {
					Delta struct{ Content string } `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &oaiChunk); err == nil && len(oaiChunk.Choices) > 0 {
				assembled += oaiChunk.Choices[0].Delta.Content
			}
		}
		if chunk.Done {
			gotDone = true
		}
	}

	if assembled != "Hello Gemini!" {
		t.Errorf("assembled = %q, want %q", assembled, "Hello Gemini!")
	}
	if !gotDone {
		t.Error("should have received DONE")
	}
}

func TestGemini_RoleTranslation(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content":      map[string]any{"parts": []map[string]string{{"text": "ok"}}, "role": "model"},
				"finishReason": "STOP",
			}},
		})
	}))
	defer server.Close()

	gem := NewGemini(ProviderConfig{APIKey: "key", BaseURL: server.URL})
	gem.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: "What is 2+2?"},
			{Role: "assistant", Content: "4"},
			{Role: "user", Content: "And 3+3?"},
		},
	})

	contents := capturedBody["contents"].([]any)
	// Second message should have role "model" (Gemini's name for assistant)
	msg2 := contents[1].(map[string]any)
	if msg2["role"] != "model" {
		t.Errorf("assistant role should be translated to 'model', got %q", msg2["role"])
	}
}

// ============================================================
// ProviderAPIError tests
// ============================================================

func TestProviderAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		status    int
		retryable bool
	}{
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{422, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
	}
	for _, tt := range tests {
		err := &ProviderAPIError{Provider: "test", StatusCode: tt.status}
		if err.IsRetryable() != tt.retryable {
			t.Errorf("status %d: IsRetryable() = %v, want %v", tt.status, err.IsRetryable(), tt.retryable)
		}
	}
}

// ============================================================
// ProviderForModel tests
// ============================================================

func TestProviderForModel_AllModels(t *testing.T) {
	tests := []struct {
		model    string
		provider string
	}{
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"o1", "openai"},
		{"o3-mini", "openai"},
		{"claude-sonnet-4-5-20250929", "anthropic"},
		{"claude-opus-4-6", "anthropic"},
		{"claude-haiku-4-5-20251001", "anthropic"},
		{"gemini-2.0-flash", "gemini"},
		{"gemini-1.5-pro", "gemini"},
		{"llama-3.3-70b-versatile", "groq"},
		{"mixtral-8x7b-32768", "groq"},
		{"unknown-model", "openai"}, // default
	}
	for _, tt := range tests {
		got := ProviderForModel(tt.model)
		if got != tt.provider {
			t.Errorf("ProviderForModel(%q) = %q, want %q", tt.model, got, tt.provider)
		}
	}
}

// ============================================================
// Anthropic error handling
// ============================================================

func TestAnthropic_StreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\"}",
			"event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}",
		}
		for _, e := range events {
			fmt.Fprintf(w, "%s\n\n", e)
			flusher.Flush()
		}
	}))
	defer server.Close()

	ant := NewAnthropic(ProviderConfig{APIKey: "key", BaseURL: server.URL})
	ch, err := ant.SendStream(context.Background(), &Request{
		Model: "claude-sonnet-4-5-20250929", Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}

	var gotError bool
	for chunk := range ch {
		if chunk.Error != nil {
			gotError = true
			if !strings.Contains(chunk.Error.Error(), "Overloaded") {
				t.Errorf("error = %v, want Overloaded", chunk.Error)
			}
		}
	}
	if !gotError {
		t.Error("should have received stream error")
	}
}

// ============================================================
// Timing / integration helpers
// ============================================================

func TestAllProviders_Timeout(t *testing.T) {
	// Server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // block
	}))
	defer server.Close()

	timeout := 100 * time.Millisecond
	req := &Request{Model: "test", Messages: []Message{{Role: "user", Content: "Hi"}}}

	providers := []Provider{
		NewOpenAI(ProviderConfig{APIKey: "k", BaseURL: server.URL + "/v1", Timeout: timeout}),
		NewAnthropic(ProviderConfig{APIKey: "k", BaseURL: server.URL, Timeout: timeout}),
		NewGroq(ProviderConfig{APIKey: "k", BaseURL: server.URL + "/openai/v1", Timeout: timeout}),
		NewGemini(ProviderConfig{APIKey: "k", BaseURL: server.URL, Timeout: timeout}),
	}

	for _, p := range providers {
		start := time.Now()
		_, err := p.Send(context.Background(), req)
		elapsed := time.Since(start)

		if err == nil {
			t.Errorf("%s: expected timeout error", p.Name())
		}
		if elapsed > 2*time.Second {
			t.Errorf("%s: took %v, should timeout in ~%v", p.Name(), elapsed, timeout)
		}
	}
}
