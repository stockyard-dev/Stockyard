package target

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	c := New(Config{URL: "http://localhost:3000"})
	if c.cfg.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", c.cfg.Timeout)
	}
	if c.URL() != "http://localhost:3000" {
		t.Errorf("URL() = %q, want %q", c.URL(), "http://localhost:3000")
	}
}

func TestNew_CustomTimeout(t *testing.T) {
	c := New(Config{URL: "http://localhost:3000", Timeout: 10 * time.Second})
	if c.cfg.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", c.cfg.Timeout)
	}
}

func TestSetMeta(t *testing.T) {
	c := New(Config{URL: "http://localhost:3000"})
	m := RequestMeta{SimID: "sim-1", ConversationID: "conv-1"}
	c.SetMeta(m)
	if c.meta.SimID != "sim-1" {
		t.Errorf("SimID = %q, want %q", c.meta.SimID, "sim-1")
	}
	if c.meta.ConversationID != "conv-1" {
		t.Errorf("ConversationID = %q, want %q", c.meta.ConversationID, "conv-1")
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		url  string
		want Format
	}{
		{"http://localhost/v1/chat/completions", FormatOpenAI},
		{"http://localhost/chat/completions", FormatOpenAI},
		{"http://localhost/api/chat", FormatSimple},
		{"http://localhost:3000", FormatSimple},
	}
	for _, tt := range tests {
		c := New(Config{URL: tt.url})
		got := c.detectFormat()
		if got != tt.want {
			t.Errorf("detectFormat(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSend_OpenAIFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("X-Stampede") != "true" {
			t.Error("missing X-Stampede header")
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Hello from target"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, Format: FormatOpenAI})
	got, err := c.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got != "Hello from target" {
		t.Errorf("Send() = %q, want %q", got, "Hello from target")
	}
}

func TestSend_SimpleFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"response": "Simple response"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, Format: FormatSimple})
	got, err := c.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got != "Simple response" {
		t.Errorf("Send() = %q, want %q", got, "Simple response")
	}
}

func TestSend_SimpleFormat_AlternateKeys(t *testing.T) {
	keys := []string{"reply", "message", "content", "text", "answer", "output"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]string{key: "value-" + key}
				json.NewEncoder(w).Encode(resp)
			}))
			defer ts.Close()

			c := New(Config{URL: ts.URL, Format: FormatSimple})
			got, err := c.Send(context.Background(), "Hi")
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}
			if got != "value-"+key {
				t.Errorf("Send() = %q, want %q", got, "value-"+key)
			}
		})
	}
}

func TestSend_SimpleFormat_PlainText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain text response"))
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, Format: FormatSimple})
	got, err := c.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got != "plain text response" {
		t.Errorf("Send() = %q, want %q", got, "plain text response")
	}
}

func TestSend_OpenAI_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, Format: FormatOpenAI})
	_, err := c.Send(context.Background(), "Hi")
	if err == nil {
		t.Error("Send() expected error for 500 response")
	}
}

func TestSend_OpenAI_NoChoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"choices": []any{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, Format: FormatOpenAI})
	_, err := c.Send(context.Background(), "Hi")
	if err == nil {
		t.Error("Send() expected error for empty choices")
	}
}

func TestAddStampedeHeaders(t *testing.T) {
	c := New(Config{URL: "http://localhost"})
	c.SetMeta(RequestMeta{SimID: "sim-1", ConversationID: "conv-1"})

	req, _ := http.NewRequest("POST", "http://localhost", nil)
	c.addStampedeHeaders(req)

	if req.Header.Get("X-Stampede") != "true" {
		t.Error("missing X-Stampede header")
	}
	if req.Header.Get("X-Stampede-Sim") != "sim-1" {
		t.Error("missing X-Stampede-Sim header")
	}
	if req.Header.Get("X-Stampede-Conversation") != "conv-1" {
		t.Error("missing X-Stampede-Conversation header")
	}
	tags := req.Header.Get("X-Stockyard-Tags")
	if tags == "" {
		t.Error("missing X-Stockyard-Tags header")
	}
}

func TestAddStampedeHeaders_NoMeta(t *testing.T) {
	c := New(Config{URL: "http://localhost"})
	req, _ := http.NewRequest("POST", "http://localhost", nil)
	c.addStampedeHeaders(req)

	if req.Header.Get("X-Stampede") != "true" {
		t.Error("missing X-Stampede header")
	}
	if req.Header.Get("X-Stampede-Sim") != "" {
		t.Error("unexpected X-Stampede-Sim header when no meta set")
	}
}

func TestTruncBody(t *testing.T) {
	short := []byte("hello")
	if truncBody(short) != "hello" {
		t.Errorf("truncBody(short) = %q", truncBody(short))
	}

	long := make([]byte, 300)
	for i := range long {
		long[i] = 'x'
	}
	got := truncBody(long)
	if len(got) > 210 { // 200 + "..."
		t.Errorf("truncBody(long) len = %d, want <= 203", len(got))
	}
}

func TestSend_AutoFormat_OpenAIFirst(t *testing.T) {
	// detectFormat checks for /v1/ in URL to decide format.
	// Use a mux so the /v1/chat/completions path is served properly.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "auto detected openai"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// URL contains /v1/ so detectFormat returns FormatOpenAI
	c := New(Config{URL: ts.URL + "/v1/chat/completions", Format: FormatAuto})
	got, err := c.Send(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got != "auto detected openai" {
		t.Errorf("Send() = %q, want %q", got, "auto detected openai")
	}
	// Second call should reuse detected format
	got2, err := c.Send(context.Background(), "Hi again")
	if err != nil {
		t.Fatalf("Send() second call error = %v", err)
	}
	if got2 != "auto detected openai" {
		t.Errorf("Send() second = %q", got2)
	}
}

func TestFormatConstants(t *testing.T) {
	if FormatAuto != "auto" {
		t.Errorf("FormatAuto = %q", FormatAuto)
	}
	if FormatOpenAI != "openai" {
		t.Errorf("FormatOpenAI = %q", FormatOpenAI)
	}
	if FormatSimple != "simple" {
		t.Errorf("FormatSimple = %q", FormatSimple)
	}
}
