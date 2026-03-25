package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	b := New(Config{ProxyURL: "http://localhost:8080"})
	if b.cfg.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", b.cfg.Timeout)
	}
	if b.ProxyURL() != "http://localhost:8080" {
		t.Errorf("ProxyURL() = %q, want %q", b.ProxyURL(), "http://localhost:8080")
	}
}

func TestNew_CustomTimeout(t *testing.T) {
	b := New(Config{ProxyURL: "http://localhost:8080", Timeout: 10 * time.Second})
	if b.cfg.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", b.cfg.Timeout)
	}
}

func TestHealthCheck_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	b := New(Config{ProxyURL: ts.URL})
	err := b.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

func TestHealthCheck_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer ts.Close()

	b := New(Config{ProxyURL: ts.URL})
	err := b.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error for 503")
	}
}

func TestHealthCheck_Unreachable(t *testing.T) {
	b := New(Config{ProxyURL: "http://127.0.0.1:1", Timeout: 1 * time.Second})
	err := b.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() expected error for unreachable")
	}
}

func TestEnrichSimulation(t *testing.T) {
	traces := []proxyTrace{
		{
			ID: "t1", Model: "gpt-4", LatencyMs: 100, CostCents: 0.5,
			CacheHit: true, GuardrailTriggered: false,
			Modules: []string{"cache", "rate_limit"},
			Tags:    map[string]string{"stampede_conversation": "conv-1"},
		},
		{
			ID: "t2", Model: "gpt-4", LatencyMs: 200, CostCents: 1.0,
			CacheHit: false, GuardrailTriggered: true, RateLimited: true,
			Error:   "timeout",
			Modules: []string{"cache", "guardrail"},
			Tags:    map[string]string{"stampede_conversation": "conv-1"},
		},
		{
			ID: "t3", Model: "gpt-3.5", LatencyMs: 50, CostCents: 0.1,
			CacheHit: false,
			Modules: []string{"cache"},
			Tags:    map[string]string{"stampede_conversation": "conv-2"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Stampede") != "true" {
			t.Error("missing X-Stampede header")
		}
		resp := struct {
			Traces []proxyTrace `json:"traces"`
		}{Traces: traces}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	b := New(Config{ProxyURL: ts.URL, AdminKey: "test-key"})
	enrichment, err := b.EnrichSimulation(context.Background(), "sim-1", []string{"conv-1", "conv-2"})
	if err != nil {
		t.Fatalf("EnrichSimulation() error = %v", err)
	}

	if enrichment.SimulationID != "sim-1" {
		t.Errorf("SimulationID = %q, want %q", enrichment.SimulationID, "sim-1")
	}
	if enrichment.TotalTraces != 3 {
		t.Errorf("TotalTraces = %d, want 3", enrichment.TotalTraces)
	}
	if enrichment.GuardrailTotal != 1 {
		t.Errorf("GuardrailTotal = %d, want 1", enrichment.GuardrailTotal)
	}
	if enrichment.RateLimitTotal != 1 {
		t.Errorf("RateLimitTotal = %d, want 1", enrichment.RateLimitTotal)
	}
	if enrichment.ProviderErrors != 1 {
		t.Errorf("ProviderErrors = %d, want 1", enrichment.ProviderErrors)
	}
	if len(enrichment.Conversations) != 2 {
		t.Fatalf("Conversations len = %d, want 2", len(enrichment.Conversations))
	}

	// Check conv-1 enrichment
	var conv1 *TraceEnrichment
	for i := range enrichment.Conversations {
		if enrichment.Conversations[i].ConversationID == "conv-1" {
			conv1 = &enrichment.Conversations[i]
			break
		}
	}
	if conv1 == nil {
		t.Fatal("conv-1 not found in enrichment")
	}
	if conv1.TotalTraces != 2 {
		t.Errorf("conv-1 TotalTraces = %d, want 2", conv1.TotalTraces)
	}
	if conv1.CacheHits != 1 {
		t.Errorf("conv-1 CacheHits = %d, want 1", conv1.CacheHits)
	}
	if conv1.CacheMisses != 1 {
		t.Errorf("conv-1 CacheMisses = %d, want 1", conv1.CacheMisses)
	}
	if conv1.AvgLatencyMs != 150 {
		t.Errorf("conv-1 AvgLatencyMs = %f, want 150", conv1.AvgLatencyMs)
	}

	// Check cache hit rate
	// conv-1: 1 hit, 1 miss; conv-2: 0 hit, 1 miss => 1/3 = 33.33%
	if enrichment.CacheHitRate < 33 || enrichment.CacheHitRate > 34 {
		t.Errorf("CacheHitRate = %f, want ~33.33", enrichment.CacheHitRate)
	}
}

func TestEnrichSimulation_DirectArray(t *testing.T) {
	traces := []proxyTrace{
		{
			ID: "t1", Model: "gpt-4", LatencyMs: 100,
			Tags: map[string]string{"stampede_conversation": "conv-1"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(traces)
	}))
	defer ts.Close()

	b := New(Config{ProxyURL: ts.URL})
	enrichment, err := b.EnrichSimulation(context.Background(), "sim-2", []string{"conv-1"})
	if err != nil {
		t.Fatalf("EnrichSimulation() error = %v", err)
	}
	if enrichment.TotalTraces != 1 {
		t.Errorf("TotalTraces = %d, want 1", enrichment.TotalTraces)
	}
}

func TestEnrichSimulation_FetchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	b := New(Config{ProxyURL: ts.URL})
	_, err := b.EnrichSimulation(context.Background(), "sim-err", []string{"conv-1"})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
