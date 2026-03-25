package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =========================================================================
// HTTPSource tests
// =========================================================================

func TestNewHTTPSource(t *testing.T) {
	s := NewHTTPSource("api", "http://example.com", map[string]string{"X-Token": "abc"})
	if s.Name() != "api" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "http" {
		t.Errorf("Type() = %q", s.Type())
	}
}

func TestHTTPSource_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "abc" {
			t.Error("missing custom header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": 42})
	}))
	defer server.Close()

	s := NewHTTPSource("api", server.URL, map[string]string{"X-Token": "abc"})
	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if snap.Source != "api" {
		t.Errorf("Source = %q", snap.Source)
	}
	if snap.Type != "http" {
		t.Errorf("Type = %q", snap.Type)
	}
	if snap.Error != "" {
		t.Errorf("unexpected error: %s", snap.Error)
	}
	if snap.Raw == nil {
		t.Error("expected parsed JSON in Raw")
	}
	if snap.Raw["status"] != "ok" {
		t.Errorf("Raw[status] = %v", snap.Raw["status"])
	}
}

func TestHTTPSource_Fetch_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	s := NewHTTPSource("api", server.URL, nil)
	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if snap.Error != "HTTP 500" {
		t.Errorf("Error = %q, want 'HTTP 500'", snap.Error)
	}
}

func TestHTTPSource_Fetch_ConnectionError(t *testing.T) {
	s := NewHTTPSource("api", "http://127.0.0.1:1", nil)
	snap, err := s.Fetch(context.Background(), nil)
	if err == nil {
		t.Error("expected error for connection failure")
	}
	if snap.Error == "" {
		t.Error("expected error in snapshot")
	}
}

// =========================================================================
// HealthSource tests
// =========================================================================

func TestNewHealthSource(t *testing.T) {
	s := NewHealthSource("health", map[string]string{"svc1": "http://localhost:1"})
	if s.Name() != "health" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "health" {
		t.Errorf("Type() = %q", s.Type())
	}
}

func TestHealthSource_Fetch(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer healthy.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer unhealthy.Close()

	s := NewHealthSource("health", map[string]string{
		"web": healthy.URL,
		"db":  unhealthy.URL,
	})

	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Raw == nil {
		t.Fatal("expected Raw data")
	}

	webInfo := snap.Raw["web"].(map[string]any)
	if webInfo["status"] != "healthy" {
		t.Errorf("web status = %v", webInfo["status"])
	}

	dbInfo := snap.Raw["db"].(map[string]any)
	if dbInfo["status"] != "unhealthy" {
		t.Errorf("db status = %v", dbInfo["status"])
	}
}

// =========================================================================
// LogSource tests
// =========================================================================

func TestNewLogSource_DefaultTail(t *testing.T) {
	s := NewLogSource("log", "/tmp/test.log", 0)
	if s.tail != 100 {
		t.Errorf("default tail = %d, want 100", s.tail)
	}
}

func TestLogSource_Fetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "log line "+string(rune('A'+i%26)))
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)

	s := NewLogSource("applog", path, 50)
	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Source != "applog" {
		t.Errorf("Source = %q", snap.Source)
	}
	if snap.Type != "log" {
		t.Errorf("Type = %q", snap.Type)
	}
	// Should have last 50 lines
	gotLines := strings.Split(snap.Data, "\n")
	if len(gotLines) != 50 {
		t.Errorf("got %d lines, want 50", len(gotLines))
	}
}

func TestLogSource_Fetch_WithHints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	content := "INFO: starting\nERROR: something failed\nINFO: done\nWARN: slow query\n"
	os.WriteFile(path, []byte(content), 0644)

	s := NewLogSource("log", path, 100)
	snap, err := s.Fetch(context.Background(), []string{"error"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(snap.Data, "ERROR") {
		t.Error("expected filtered results to contain ERROR")
	}
	if strings.Contains(snap.Data, "WARN") {
		t.Error("expected filtered results to not contain WARN")
	}
}

func TestLogSource_Fetch_MissingFile(t *testing.T) {
	s := NewLogSource("log", "/nonexistent/file.log", 100)
	_, err := s.Fetch(context.Background(), nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// =========================================================================
// EnvSource tests
// =========================================================================

func TestEnvSource_Basic(t *testing.T) {
	s := NewEnvSource("env", "", nil)
	if s.Name() != "env" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "env" {
		t.Errorf("Type() = %q", s.Type())
	}
}

func TestEnvSource_Fetch_WithPrefix(t *testing.T) {
	os.Setenv("MYAPP_DEBUG", "true")
	os.Setenv("MYAPP_PORT", "8080")
	defer os.Unsetenv("MYAPP_DEBUG")
	defer os.Unsetenv("MYAPP_PORT")

	s := NewEnvSource("env", "MYAPP_", nil)
	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Raw["MYAPP_DEBUG"] != "true" {
		t.Errorf("MYAPP_DEBUG = %v", snap.Raw["MYAPP_DEBUG"])
	}
	if snap.Raw["MYAPP_PORT"] != "8080" {
		t.Errorf("MYAPP_PORT = %v", snap.Raw["MYAPP_PORT"])
	}
}

func TestEnvSource_Fetch_RedactsSensitive(t *testing.T) {
	os.Setenv("MYAPP_API_KEY", "secret123")
	defer os.Unsetenv("MYAPP_API_KEY")

	s := NewEnvSource("env", "MYAPP_", nil)
	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Raw["MYAPP_API_KEY"] != "[REDACTED]" {
		t.Errorf("expected redacted, got %v", snap.Raw["MYAPP_API_KEY"])
	}
}

func TestEnvSource_Fetch_Include(t *testing.T) {
	os.Setenv("SPECIAL_VAR", "hello")
	defer os.Unsetenv("SPECIAL_VAR")

	s := NewEnvSource("env", "NONEXISTENT_", []string{"SPECIAL_VAR"})
	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Raw["SPECIAL_VAR"] != "hello" {
		t.Errorf("SPECIAL_VAR = %v", snap.Raw["SPECIAL_VAR"])
	}
}

// =========================================================================
// StaticSource tests
// =========================================================================

func TestStaticSource(t *testing.T) {
	s := NewStaticSource("arch", "This is a microservices architecture.")
	if s.Name() != "arch" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "static" {
		t.Errorf("Type() = %q", s.Type())
	}

	snap, err := s.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Data != "This is a microservices architecture." {
		t.Errorf("Data = %q", snap.Data)
	}
	if snap.Source != "arch" {
		t.Errorf("Source = %q", snap.Source)
	}
}

// =========================================================================
// Database connector tests (unit tests for helpers)
// =========================================================================

func TestFormatSQLValue(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"nil", nil, "NULL"},
		{"bytes", []byte("hello"), "hello"},
		{"time", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), "2024-01-15 10:30:00"},
		{"string", "text", "text"},
		{"int", 42, "42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSQLValue(tc.val)
			if got != tc.want {
				t.Errorf("formatSQLValue(%v) = %q, want %q", tc.val, got, tc.want)
			}
		})
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short", "hi", 10, "hi"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hell~"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateStr(tc.s, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.s, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestNewSQLSource(t *testing.T) {
	cfg := SQLSourceConfig{
		DSN:     "test.db",
		Driver:  "sqlite",
		Queries: map[string]string{"count": "SELECT COUNT(*) FROM users"},
	}
	s := NewSQLSource("db", cfg)
	if s.Name() != "db" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "sql" {
		t.Errorf("Type() = %q", s.Type())
	}
}

func TestNewRedisSource_Defaults(t *testing.T) {
	s := NewRedisSource("cache", RedisSourceConfig{})
	if s.Name() != "cache" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "redis" {
		t.Errorf("Type() = %q", s.Type())
	}
	if s.config.Addr != "localhost:6379" {
		t.Errorf("default addr = %q", s.config.Addr)
	}
	if len(s.config.Patterns) != 1 || s.config.Patterns[0] != "*" {
		t.Errorf("default patterns = %v", s.config.Patterns)
	}
}

// =========================================================================
// Prometheus connector tests
// =========================================================================

func TestNewPrometheusSource_DefaultQueries(t *testing.T) {
	s := NewPrometheusSource("prom", PrometheusSourceConfig{Endpoint: "http://localhost:9090"})
	if s.Name() != "prom" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.Type() != "prometheus" {
		t.Errorf("Type() = %q", s.Type())
	}
	// Should have default queries
	if len(s.config.Queries) == 0 {
		t.Error("expected default queries")
	}
}

func TestNewPrometheusSource_CustomQueries(t *testing.T) {
	s := NewPrometheusSource("prom", PrometheusSourceConfig{
		Endpoint: "http://localhost:9090",
		Queries:  map[string]string{"custom": "up"},
	})
	if len(s.config.Queries) != 1 {
		t.Errorf("expected 1 custom query, got %d", len(s.config.Queries))
	}
}

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		val  string
		want string
	}{
		{"NaN", "NaN"},
		{"+Inf", "+Inf"},
		{"-Inf", "-Inf"},
		{"42.5", "42.5"},
	}
	for _, tc := range tests {
		got := formatMetricValue(tc.val)
		if got != tc.want {
			t.Errorf("formatMetricValue(%q) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestDefaultPromQueries(t *testing.T) {
	queries := defaultPromQueries()
	expected := []string{"error_rate", "p99_latency", "request_rate", "memory_usage"}
	for _, name := range expected {
		if _, ok := queries[name]; !ok {
			t.Errorf("missing default query %q", name)
		}
	}
}

func TestRedisSource_ParseArrayReply(t *testing.T) {
	s := &RedisSource{}
	reply := "*3\r\n$3\r\nfoo\r\n$3\r\nbar\r\n$3\r\nbaz\r\n"
	got := s.parseArrayReply(reply, 10)
	if len(got) != 3 {
		t.Errorf("expected 3 keys, got %d: %v", len(got), got)
	}

	// Test with max limit
	got = s.parseArrayReply(reply, 2)
	if len(got) != 2 {
		t.Errorf("expected 2 keys with limit, got %d", len(got))
	}
}

func TestRedisSource_ParseBulkReply(t *testing.T) {
	s := &RedisSource{}

	tests := []struct {
		name  string
		reply string
		want  string
	}{
		{"normal", "$5\r\nhello\r\n", "hello"},
		{"nil", "$-1\r\n", "(nil)"},
		{"short", "x", "x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.parseBulkReply(tc.reply)
			if got != tc.want {
				t.Errorf("parseBulkReply(%q) = %q, want %q", tc.reply, got, tc.want)
			}
		})
	}
}

func TestPrometheusSource_FormatResult(t *testing.T) {
	s := &PrometheusSource{name: "test"}
	resp := &promResponse{
		Status: "success",
		Data: promData{
			ResultType: "vector",
			Result: []promResult{
				{
					Metric: map[string]string{"__name__": "up", "instance": "localhost:9090"},
					Value:  []any{float64(1234567890), "1"},
				},
			},
		},
	}

	output := s.formatResult("uptime", "up", resp)
	if !strings.Contains(output, "--- uptime ---") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "up") {
		t.Error("missing metric name")
	}
}

func TestPrometheusSource_FormatResult_NoData(t *testing.T) {
	s := &PrometheusSource{name: "test"}
	resp := &promResponse{
		Data: promData{Result: nil},
	}
	output := s.formatResult("empty", "up", resp)
	if !strings.Contains(output, "(no data)") {
		t.Error("expected '(no data)' for empty result")
	}
}

// =========================================================================
// Snapshot tests
// =========================================================================

func TestSnapshot_Fields(t *testing.T) {
	snap := Snapshot{
		Source:    "test",
		Type:     "http",
		FetchedAt: time.Now(),
		Data:     "test data",
		LatencyMs: 42,
	}
	if snap.Source != "test" {
		t.Errorf("Source = %q", snap.Source)
	}
	if snap.LatencyMs != 42 {
		t.Errorf("LatencyMs = %d", snap.LatencyMs)
	}
}
