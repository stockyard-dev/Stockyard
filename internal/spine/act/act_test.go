package act

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/decide"
)

// mockHandler implements Handler for testing.
type mockHandler struct {
	executeCalled  bool
	rollbackCalled bool
	executeErr     error
	rollbackErr    error
}

func (h *mockHandler) Execute(ctx context.Context, d decide.Decision) error {
	h.executeCalled = true
	return h.executeErr
}

func (h *mockHandler) Rollback(ctx context.Context, d decide.Decision) error {
	h.rollbackCalled = true
	return h.rollbackErr
}

func TestNewExecutor(t *testing.T) {
	e := NewExecutor(Config{})
	if e.maxLog != 500 {
		t.Errorf("default maxLog = %d, want 500", e.maxLog)
	}
	if e.dryRun {
		t.Error("dryRun should be false by default")
	}
}

func TestNewExecutor_CustomMaxLog(t *testing.T) {
	e := NewExecutor(Config{MaxLog: 100})
	if e.maxLog != 100 {
		t.Errorf("maxLog = %d, want 100", e.maxLog)
	}
}

func TestExecutor_RegisterHandler(t *testing.T) {
	e := NewExecutor(Config{})
	h := &mockHandler{}
	e.RegisterHandler(decide.ActionScaleUp, h)

	e.mu.RLock()
	_, ok := e.handlers[decide.ActionScaleUp]
	e.mu.RUnlock()
	if !ok {
		t.Error("handler not registered")
	}
}

func TestExecutor_Execute_DryRun(t *testing.T) {
	e := NewExecutor(Config{DryRun: true})
	decisions := []decide.Decision{
		{ID: "d1", Action: decide.ActionScaleUp, Reason: "high load"},
	}

	logs := e.Execute(context.Background(), decisions)
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Executed {
		t.Error("should not be executed in dry run")
	}
	if logs[0].Result == "" {
		t.Error("should have result message")
	}
}

func TestExecutor_Execute_Success(t *testing.T) {
	h := &mockHandler{}
	e := NewExecutor(Config{})
	e.RegisterHandler(decide.ActionScaleUp, h)

	decisions := []decide.Decision{
		{ID: "d1", Action: decide.ActionScaleUp, Reason: "high load"},
	}
	logs := e.Execute(context.Background(), decisions)

	if !h.executeCalled {
		t.Error("handler not called")
	}
	if !logs[0].Executed {
		t.Error("should be marked as executed")
	}
	if logs[0].Result != "success" {
		t.Errorf("Result = %q, want success", logs[0].Result)
	}
}

func TestExecutor_Execute_HandlerError(t *testing.T) {
	h := &mockHandler{executeErr: fmt.Errorf("scale failed")}
	e := NewExecutor(Config{})
	e.RegisterHandler(decide.ActionScaleUp, h)

	decisions := []decide.Decision{
		{ID: "d1", Action: decide.ActionScaleUp},
	}
	logs := e.Execute(context.Background(), decisions)

	if logs[0].Executed {
		t.Error("should not be marked as executed on error")
	}
	if logs[0].Error != "scale failed" {
		t.Errorf("Error = %q", logs[0].Error)
	}
}

func TestExecutor_Execute_NoHandler(t *testing.T) {
	e := NewExecutor(Config{})
	decisions := []decide.Decision{
		{ID: "d1", Action: decide.ActionScaleUp},
	}
	logs := e.Execute(context.Background(), decisions)

	if logs[0].Executed {
		t.Error("should not be executed without handler")
	}
	if logs[0].Error == "" {
		t.Error("should have error message")
	}
}

func TestExecutor_Execute_OnActionCallback(t *testing.T) {
	var mu sync.Mutex
	var called []ActionLog
	e := NewExecutor(Config{
		OnAction: func(log ActionLog) {
			mu.Lock()
			called = append(called, log)
			mu.Unlock()
		},
	})
	h := &mockHandler{}
	e.RegisterHandler(decide.ActionAlert, h)

	decisions := []decide.Decision{
		{ID: "d1", Action: decide.ActionAlert},
	}
	e.Execute(context.Background(), decisions)

	mu.Lock()
	defer mu.Unlock()
	if len(called) != 1 {
		t.Errorf("callback called %d times, want 1", len(called))
	}
}

func TestExecutor_RecentActions(t *testing.T) {
	e := NewExecutor(Config{})
	h := &mockHandler{}
	e.RegisterHandler(decide.ActionAlert, h)

	for i := 0; i < 5; i++ {
		e.Execute(context.Background(), []decide.Decision{
			{ID: fmt.Sprintf("d%d", i), Action: decide.ActionAlert},
		})
	}

	recent := e.RecentActions(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent actions, got %d", len(recent))
	}

	// More than available
	recent = e.RecentActions(10)
	if len(recent) != 5 {
		t.Errorf("expected 5 recent actions, got %d", len(recent))
	}
}

func TestExecutor_MaxLogTruncation(t *testing.T) {
	e := NewExecutor(Config{MaxLog: 3})
	h := &mockHandler{}
	e.RegisterHandler(decide.ActionAlert, h)

	for i := 0; i < 10; i++ {
		e.Execute(context.Background(), []decide.Decision{
			{ID: fmt.Sprintf("d%d", i), Action: decide.ActionAlert},
		})
	}

	recent := e.RecentActions(100)
	if len(recent) != 3 {
		t.Errorf("expected max 3 log entries, got %d", len(recent))
	}
}

func TestLogHandler(t *testing.T) {
	h := &LogHandler{}
	d := decide.Decision{Action: decide.ActionAlert, Reason: "test", Urgency: decide.UrgencyLow}

	if err := h.Execute(context.Background(), d); err != nil {
		t.Errorf("Execute: %v", err)
	}
	if err := h.Rollback(context.Background(), d); err != nil {
		t.Errorf("Rollback: %v", err)
	}
}

func TestWebhookHandler(t *testing.T) {
	h := &WebhookHandler{URL: "http://example.com/hook"}
	d := decide.Decision{Action: decide.ActionAlert, Reason: "test"}

	if err := h.Execute(context.Background(), d); err != nil {
		t.Errorf("Execute: %v", err)
	}
	if err := h.Rollback(context.Background(), d); err != nil {
		t.Errorf("Rollback: %v", err)
	}
}

func TestNewScriptHandler(t *testing.T) {
	scripts := map[string]string{"scale_up": "echo hello"}
	h := NewScriptHandler(scripts)
	if h.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v", h.Timeout)
	}
	if h.Scripts["scale_up"] != "echo hello" {
		t.Error("scripts not set")
	}
}

func TestScriptHandler_Execute(t *testing.T) {
	h := NewScriptHandler(map[string]string{
		"alert": "echo test",
	})
	d := decide.Decision{ID: "d1", Action: decide.ActionAlert, Urgency: decide.UrgencyLow, Reason: "test"}

	err := h.Execute(context.Background(), d)
	if err != nil {
		t.Errorf("Execute: %v", err)
	}
}

func TestScriptHandler_Execute_MissingScript(t *testing.T) {
	h := NewScriptHandler(map[string]string{})
	d := decide.Decision{Action: decide.ActionScaleUp}

	err := h.Execute(context.Background(), d)
	if err == nil {
		t.Error("expected error for missing script")
	}
}

func TestScriptHandler_Execute_WithParams(t *testing.T) {
	h := NewScriptHandler(map[string]string{
		"scale_up": "echo $SPINE_PARAM_INSTANCES_DELTA",
	})
	d := decide.Decision{
		ID:     "d1",
		Action: decide.ActionScaleUp,
		Params: map[string]any{"instances_delta": 2},
	}
	err := h.Execute(context.Background(), d)
	if err != nil {
		t.Errorf("Execute: %v", err)
	}
}

func TestScriptHandler_Rollback_MissingScript(t *testing.T) {
	h := NewScriptHandler(map[string]string{})
	d := decide.Decision{Action: decide.ActionScaleUp}

	err := h.Rollback(context.Background(), d)
	if err == nil {
		t.Error("expected error for missing rollback script")
	}
}

func TestScriptHandler_Rollback(t *testing.T) {
	h := NewScriptHandler(map[string]string{
		"scale_up_rollback": "echo rollback",
	})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp}
	err := h.Rollback(context.Background(), d)
	if err != nil {
		t.Errorf("Rollback: %v", err)
	}
}

func TestNewDockerHandler(t *testing.T) {
	h := NewDockerHandler("web", "/path/to/docker-compose.yml")
	if h.ServiceName != "web" {
		t.Errorf("ServiceName = %q", h.ServiceName)
	}
	if h.ComposeFile != "/path/to/docker-compose.yml" {
		t.Errorf("ComposeFile = %q", h.ComposeFile)
	}
}

func TestNewHTTPHandler(t *testing.T) {
	h := NewHTTPHandler("http://example.com/hook", map[string]string{"Authorization": "Bearer tok"})
	if h.WebhookURL != "http://example.com/hook" {
		t.Errorf("WebhookURL = %q", h.WebhookURL)
	}
	if h.Headers["Authorization"] != "Bearer tok" {
		t.Error("headers not set")
	}
	if h.client == nil {
		t.Error("client should be initialized")
	}
}

func TestHTTPHandler_Execute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != "Stockyard-Spine/1.0" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("X-Custom") != "val" {
			t.Errorf("X-Custom = %q", r.Header.Get("X-Custom"))
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		if payload["event"] != "spine.action" {
			t.Errorf("event = %v", payload["event"])
		}

		w.WriteHeader(200)
	}))
	defer server.Close()

	h := NewHTTPHandler(server.URL, map[string]string{"X-Custom": "val"})
	d := decide.Decision{
		ID:        "d1",
		Action:    decide.ActionScaleUp,
		Urgency:   decide.UrgencyHigh,
		Reason:    "load spike",
		Impact:    "add 1 instance",
		Rollback:  "remove instance",
		Timestamp: time.Now(),
		Params:    map[string]any{"count": 1},
	}

	err := h.Execute(context.Background(), d)
	if err != nil {
		t.Errorf("Execute: %v", err)
	}
}

func TestHTTPHandler_Execute_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	h := NewHTTPHandler(server.URL, nil)
	d := decide.Decision{ID: "d1", Action: decide.ActionAlert, Timestamp: time.Now()}

	err := h.Execute(context.Background(), d)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestHTTPHandler_Rollback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		json.Unmarshal(body, &payload)
		if payload["event"] != "spine.rollback" {
			t.Errorf("event = %v", payload["event"])
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	h := NewHTTPHandler(server.URL, nil)
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp}

	err := h.Rollback(context.Background(), d)
	if err != nil {
		t.Errorf("Rollback: %v", err)
	}
}
