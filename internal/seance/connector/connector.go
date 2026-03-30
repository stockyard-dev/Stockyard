// Package connector provides pluggable adapters for querying live system state.
// Each connector knows how to fetch data from one kind of source: HTTP endpoints,
// databases, log files, metrics APIs, queue depths, config stores.
package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Source represents a queryable data source.
type Source interface {
	Name() string
	Type() string
	Fetch(ctx context.Context, hints []string) (*Snapshot, error)
}

// Snapshot holds data collected from a single source at a point in time.
type Snapshot struct {
	Source    string         `json:"source"`
	Type     string         `json:"type"`
	FetchedAt time.Time     `json:"fetched_at"`
	Data     string         `json:"data"`          // human-readable summary
	Raw      map[string]any `json:"raw,omitempty"` // structured data if available
	Error    string         `json:"error,omitempty"`
	LatencyMs int           `json:"latency_ms"`
}

// =========================================================================
// HTTP Endpoint connector — fetches JSON from any URL
// =========================================================================

// HTTPSource queries an HTTP endpoint.
type HTTPSource struct {
	name    string
	url     string
	headers map[string]string
	client  *http.Client
}

// NewHTTPSource creates an HTTP connector.
func NewHTTPSource(name, url string, headers map[string]string) *HTTPSource {
	return &HTTPSource{
		name:    name,
		url:     url,
		headers: headers,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *HTTPSource) Name() string { return s.name }
func (s *HTTPSource) Type() string { return "http" }

func (s *HTTPSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		return &Snapshot{Source: s.name, Type: "http", Error: err.Error(), FetchedAt: time.Now()}, err
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	snap := &Snapshot{
		Source:    s.name,
		Type:     "http",
		FetchedAt: time.Now(),
		LatencyMs: latency,
	}

	if err != nil {
		snap.Error = err.Error()
		return snap, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		snap.Error = fmt.Sprintf("read body: %v", err)
		return snap, err
	}
	snap.Data = string(body)

	// Try to parse as JSON for structured access
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) == nil {
		snap.Raw = parsed
	}

	if resp.StatusCode >= 400 {
		snap.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return snap, nil
}

// =========================================================================
// Health connector — probes multiple health endpoints
// =========================================================================

// HealthSource checks health of multiple services.
type HealthSource struct {
	name      string
	endpoints map[string]string // service name -> URL
	client    *http.Client
}

// NewHealthSource creates a multi-service health checker.
func NewHealthSource(name string, endpoints map[string]string) *HealthSource {
	return &HealthSource{
		name:      name,
		endpoints: endpoints,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *HealthSource) Name() string { return s.name }
func (s *HealthSource) Type() string { return "health" }

func (s *HealthSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	start := time.Now()
	results := map[string]any{}
	var summary []string

	for svc, url := range s.endpoints {
		reqStart := time.Now()
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		resp, err := s.client.Do(req)
		latency := time.Since(reqStart).Milliseconds()

		if err != nil {
			results[svc] = map[string]any{"status": "down", "error": err.Error(), "latency_ms": latency}
			summary = append(summary, fmt.Sprintf("%s: DOWN (%v)", svc, err))
		} else {
			resp.Body.Close()
			status := "healthy"
			if resp.StatusCode >= 400 {
				status = "unhealthy"
			}
			results[svc] = map[string]any{"status": status, "code": resp.StatusCode, "latency_ms": latency}
			summary = append(summary, fmt.Sprintf("%s: %s (%d, %dms)", svc, status, resp.StatusCode, latency))
		}
	}

	return &Snapshot{
		Source:    s.name,
		Type:     "health",
		FetchedAt: time.Now(),
		Data:     strings.Join(summary, "\n"),
		Raw:      results,
		LatencyMs: int(time.Since(start).Milliseconds()),
	}, nil
}

// =========================================================================
// Log file connector — tails recent log entries
// =========================================================================

// LogSource reads recent lines from a log file.
type LogSource struct {
	name string
	path string
	tail int // number of lines to read from end
}

// NewLogSource creates a log file connector.
func NewLogSource(name, path string, tail int) *LogSource {
	if tail <= 0 {
		tail = 100
	}
	return &LogSource{name: name, path: path, tail: tail}
}

func (s *LogSource) Name() string { return s.name }
func (s *LogSource) Type() string { return "log" }

func (s *LogSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	start := time.Now()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return &Snapshot{Source: s.name, Type: "log", Error: err.Error(), FetchedAt: time.Now()}, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > s.tail {
		lines = lines[len(lines)-s.tail:]
	}

	// Filter by hints (keywords) if provided
	if len(hints) > 0 {
		var filtered []string
		for _, line := range lines {
			for _, hint := range hints {
				if strings.Contains(strings.ToLower(line), strings.ToLower(hint)) {
					filtered = append(filtered, line)
					break
				}
			}
		}
		if len(filtered) > 0 {
			lines = filtered
		}
	}

	content := strings.Join(lines, "\n")
	if len(content) > 50000 {
		content = content[len(content)-50000:]
	}

	return &Snapshot{
		Source:    s.name,
		Type:     "log",
		FetchedAt: time.Now(),
		Data:     content,
		LatencyMs: int(time.Since(start).Milliseconds()),
	}, nil
}

// =========================================================================
// Environment connector — reads env vars and config
// =========================================================================

// EnvSource captures current environment and configuration.
type EnvSource struct {
	name    string
	prefix  string   // only capture vars with this prefix
	include []string // specific vars to include
}

// NewEnvSource creates an environment connector.
func NewEnvSource(name, prefix string, include []string) *EnvSource {
	return &EnvSource{name: name, prefix: prefix, include: include}
}

func (s *EnvSource) Name() string { return s.name }
func (s *EnvSource) Type() string { return "env" }

func (s *EnvSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	includeSet := map[string]bool{}
	for _, v := range s.include {
		includeSet[v] = true
	}

	vars := map[string]any{}
	var lines []string

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		// Filter
		include := false
		if s.prefix != "" && strings.HasPrefix(key, s.prefix) {
			include = true
		}
		if includeSet[key] {
			include = true
		}
		if !include && s.prefix != "" {
			continue
		}

		// Redact sensitive values
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, "key") || strings.Contains(keyLower, "secret") ||
			strings.Contains(keyLower, "password") || strings.Contains(keyLower, "token") {
			val = "[REDACTED]"
		}

		vars[key] = val
		lines = append(lines, fmt.Sprintf("%s=%s", key, val))
	}

	return &Snapshot{
		Source:    s.name,
		Type:     "env",
		FetchedAt: time.Now(),
		Data:     strings.Join(lines, "\n"),
		Raw:      vars,
	}, nil
}

// =========================================================================
// Static context connector — provides hardcoded context about the system
// =========================================================================

// StaticSource provides fixed context (architecture docs, service descriptions, etc.).
type StaticSource struct {
	name    string
	content string
}

// NewStaticSource creates a static context source.
func NewStaticSource(name, content string) *StaticSource {
	return &StaticSource{name: name, content: content}
}

func (s *StaticSource) Name() string { return s.name }
func (s *StaticSource) Type() string { return "static" }

func (s *StaticSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	return &Snapshot{
		Source:    s.name,
		Type:     "static",
		FetchedAt: time.Now(),
		Data:     s.content,
	}, nil
}
