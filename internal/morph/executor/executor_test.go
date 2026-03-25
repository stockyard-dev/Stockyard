package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		params   map[string]any
		want     string
	}{
		{
			name:     "string param",
			template: "/v1/customers/{{customer_id}}",
			params:   map[string]any{"customer_id": "cus_123"},
			want:     "/v1/customers/cus_123",
		},
		{
			name:     "integer float64 param",
			template: `{"amount":{{amount}}}`,
			params:   map[string]any{"amount": float64(5000)},
			want:     `{"amount":5000}`,
		},
		{
			name:     "fractional float64 param",
			template: `{"rate":{{rate}}}`,
			params:   map[string]any{"rate": float64(0.15)},
			want:     `{"rate":0.15}`,
		},
		{
			name:     "bool param",
			template: `{"active":{{active}}}`,
			params:   map[string]any{"active": true},
			want:     `{"active":true}`,
		},
		{
			name:     "multiple params",
			template: "/repos/{{owner}}/{{repo}}/pulls",
			params:   map[string]any{"owner": "acme", "repo": "widget"},
			want:     "/repos/acme/widget/pulls",
		},
		{
			name:     "no params",
			template: "/v1/charges",
			params:   map[string]any{},
			want:     "/v1/charges",
		},
		{
			name:     "missing param left as-is",
			template: "/v1/{{resource}}/{{id}}",
			params:   map[string]any{"resource": "charges"},
			want:     "/v1/charges/{{id}}",
		},
		{
			name:     "json marshal fallback",
			template: `{"items":{{items}}}`,
			params:   map[string]any{"items": []string{"a", "b"}},
			want:     `{"items":["a","b"]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTemplate(tc.template, tc.params)
			if got != tc.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTruncBody(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncBody([]byte(tc.input), tc.n)
			if got != tc.want {
				t.Errorf("truncBody() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveStepRefs(t *testing.T) {
	results := map[int]*Result{
		0: {Response: map[string]any{"id": "abc123", "name": "test"}},
		1: {Response: map[string]any{"status": "active"}},
	}

	tests := []struct {
		name   string
		params map[string]any
		want   map[string]any
	}{
		{
			name:   "simple ref",
			params: map[string]any{"customer": "{{step_0.id}}"},
			want:   map[string]any{"customer": "abc123"},
		},
		{
			name:   "no ref",
			params: map[string]any{"name": "plain"},
			want:   map[string]any{"name": "plain"},
		},
		{
			name:   "multiple refs",
			params: map[string]any{"msg": "{{step_0.id}} is {{step_1.status}}"},
			want:   map[string]any{"msg": "abc123 is active"},
		},
		{
			name:   "non-string values unchanged",
			params: map[string]any{"count": 42},
			want:   map[string]any{"count": 42},
		},
		{
			name:   "missing field resolves to empty",
			params: map[string]any{"x": "{{step_0.missing}}"},
			want:   map[string]any{"x": ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveStepRefs(tc.params, results)
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("key %q: got %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestResolveStringNilResult(t *testing.T) {
	results := map[int]*Result{
		0: nil,
	}
	got := resolveString("{{step_0.id}}", results)
	if got != "{{step_0.id}}" {
		t.Errorf("expected unresolved ref with nil result, got %q", got)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want 1s", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", cfg.MaxDelay)
	}
	expected := []int{429, 500, 502, 503, 504}
	for _, code := range expected {
		if !cfg.RetryableStatuses[code] {
			t.Errorf("expected %d to be retryable", code)
		}
	}
	if cfg.RetryableStatuses[200] {
		t.Error("200 should not be retryable")
	}
}

func TestRateLimitsUpdateAndGet(t *testing.T) {
	rl := newRateLimits()

	// nil response should not panic
	rl.update("svc", nil)
	if info := rl.get("svc"); info != nil {
		t.Errorf("expected nil info for unknown service")
	}

	// Update with headers
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("X-RateLimit-Remaining", "42")
	resp.Header.Set("X-RateLimit-Reset", "1700000000")
	rl.update("github", resp)

	info := rl.get("github")
	if info == nil {
		t.Fatal("expected rate limit info")
	}
	if info.Remaining != 42 {
		t.Errorf("Remaining = %d, want 42", info.Remaining)
	}
	if info.ResetAt != time.Unix(1700000000, 0) {
		t.Errorf("ResetAt = %v, want %v", info.ResetAt, time.Unix(1700000000, 0))
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "5", 5 * time.Second},
		{"invalid", "abc", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tc.value != "" {
				resp.Header.Set("Retry-After", tc.value)
			}
			got := parseRetryAfter(resp)
			if got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestExecuteWithRetry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := server.Client()
	req, _ := http.NewRequest("GET", server.URL, nil)

	cfg := &RetryConfig{
		MaxRetries:        1,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		RetryableStatuses: map[int]bool{500: true},
	}

	resp, err := executeWithRetry(client, req, cfg, "test", newRateLimits())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestExecuteWithRetry_NilConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := server.Client()
	req, _ := http.NewRequest("GET", server.URL, nil)

	resp, err := executeWithRetry(client, req, nil, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestExecuteWithRetry_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Header().Set("Retry-After", "60")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := server.Client()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)

	cfg := &RetryConfig{
		MaxRetries:        2,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		RetryableStatuses: map[int]bool{429: true},
	}

	_, err := executeWithRetry(client, req, cfg, "test", newRateLimits())
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestMultiResult(t *testing.T) {
	mr := &MultiResult{
		Steps:   []Result{{Service: "a", Success: true}, {Service: "b", Success: false}},
		Success: false,
	}
	if mr.Success {
		t.Error("expected Success=false")
	}
	if len(mr.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(mr.Steps))
	}
}
