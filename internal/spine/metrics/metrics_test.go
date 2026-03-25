package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

func TestNewPrometheusSource(t *testing.T) {
	p := NewPrometheusSource("http://localhost:9090")
	if p.Endpoint != "http://localhost:9090" {
		t.Errorf("Endpoint = %q", p.Endpoint)
	}
	if p.client == nil {
		t.Error("client should be initialized")
	}
}

func TestPrometheusSource_Query(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := promQueryResult{
			Status: "success",
		}
		resp.Data.ResultType = "vector"
		resp.Data.Result = []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		}{
			{
				Metric: map[string]string{"__name__": "cpu"},
				Value:  [2]interface{}{float64(1234567890), "42.5"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewPrometheusSource(server.URL)
	val, err := p.query("test_query")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if val != 42.5 {
		t.Errorf("value = %f, want 42.5", val)
	}
}

func TestPrometheusSource_Query_NoData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := promQueryResult{Status: "success"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewPrometheusSource(server.URL)
	_, err := p.query("empty_query")
	if err == nil {
		t.Error("expected error for empty result")
	}
}

func TestPrometheusSource_Query_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := promQueryResult{Status: "error"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewPrometheusSource(server.URL)
	_, err := p.query("error_query")
	if err == nil {
		t.Error("expected error for error status")
	}
}

func TestPrometheusSource_Collect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := promQueryResult{Status: "success"}
		resp.Data.ResultType = "vector"
		resp.Data.Result = []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		}{
			{Value: [2]interface{}{float64(0), "25.0"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewPrometheusSource(server.URL)
	m, err := p.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil metrics")
	}
	// CPUPercent should be set from the first query
	if m.CPUPercent != 25.0 {
		t.Errorf("CPUPercent = %f, want 25.0", m.CPUPercent)
	}
}

func TestNewHealthProbeCollector(t *testing.T) {
	h := NewHealthProbeCollector("http://localhost:8080/metrics")
	if h.URL != "http://localhost:8080/metrics" {
		t.Errorf("URL = %q", h.URL)
	}
}

func TestHealthProbeCollector_Collect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := SystemMetrics{
			CPUPercent:    55.0,
			MemoryPercent: 70.0,
			Instances:     3,
			RPS:           1200.5,
			ErrorRate:     0.5,
		}
		json.NewEncoder(w).Encode(m)
	}))
	defer server.Close()

	h := NewHealthProbeCollector(server.URL)
	m, err := h.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if m.CPUPercent != 55.0 {
		t.Errorf("CPUPercent = %f", m.CPUPercent)
	}
	if m.MemoryPercent != 70.0 {
		t.Errorf("MemoryPercent = %f", m.MemoryPercent)
	}
	if m.Instances != 3 {
		t.Errorf("Instances = %d", m.Instances)
	}
}

func TestHealthProbeCollector_Collect_Error(t *testing.T) {
	h := NewHealthProbeCollector("http://127.0.0.1:1/metrics")
	_, err := h.Collect()
	if err == nil {
		t.Error("expected error for unreachable endpoint")
	}
}

func TestHealthProbeCollector_Collect_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	h := NewHealthProbeCollector(server.URL)
	_, err := h.Collect()
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestApplyToCurrentMetrics(t *testing.T) {
	sys := &SystemMetrics{
		CPUPercent:    80.0,
		MemoryPercent: 65.0,
		Instances:     4,
		RPS:           500,
	}
	cm := &objectives.CurrentMetrics{}

	ApplyToCurrentMetrics(sys, cm)

	if cm.CPUUsage != 80.0 {
		t.Errorf("CPUUsage = %f", cm.CPUUsage)
	}
	if cm.MemoryUsage != 65.0 {
		t.Errorf("MemoryUsage = %f", cm.MemoryUsage)
	}
	if cm.Instances != 4 {
		t.Errorf("Instances = %d", cm.Instances)
	}
	if cm.CurrentRPS != 500 {
		t.Errorf("CurrentRPS = %d", cm.CurrentRPS)
	}
}

func TestApplyToCurrentMetrics_NilInputs(t *testing.T) {
	// Should not panic
	ApplyToCurrentMetrics(nil, &objectives.CurrentMetrics{})
	ApplyToCurrentMetrics(&SystemMetrics{}, nil)
	ApplyToCurrentMetrics(nil, nil)
}

func TestApplyToCurrentMetrics_ZeroRPS(t *testing.T) {
	sys := &SystemMetrics{RPS: 0}
	cm := &objectives.CurrentMetrics{CurrentRPS: 100}
	ApplyToCurrentMetrics(sys, cm)
	// Should not override when RPS is 0
	if cm.CurrentRPS != 100 {
		t.Errorf("CurrentRPS = %d, want 100 (should not override with 0)", cm.CurrentRPS)
	}
}
