// Package metrics provides system metrics collection for Spine.
// Supports both Prometheus scraping and direct health-probe collection.
package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

// MetricsCollector is the interface for gathering system metrics.
// Observers can use any implementation: health probes, Prometheus, etc.
type MetricsCollector interface {
	// Collect gathers current system metrics from the source.
	Collect() (*SystemMetrics, error)
}

// SystemMetrics holds resource-level metrics separate from probe-derived metrics.
type SystemMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	Instances     int     `json:"instances"`
	RPS           float64 `json:"rps"`
	ErrorRate     float64 `json:"error_rate"`
	CollectedAt   time.Time `json:"collected_at"`
}

// PrometheusSource scrapes metrics from a Prometheus-compatible API endpoint.
type PrometheusSource struct {
	Endpoint string       // e.g. http://localhost:9090
	client   *http.Client
}

// promQueryResult is the Prometheus /api/v1/query response envelope.
type promQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"` // [timestamp, "value"]
		} `json:"result"`
	} `json:"data"`
}

// NewPrometheusSource creates a Prometheus metrics collector.
func NewPrometheusSource(endpoint string) *PrometheusSource {
	return &PrometheusSource{
		Endpoint: endpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Collect queries Prometheus for CPU, memory, RPS, and error rate.
func (p *PrometheusSource) Collect() (*SystemMetrics, error) {
	m := &SystemMetrics{CollectedAt: time.Now()}

	// CPU: rate of process CPU seconds over 5 minutes, as percentage
	cpuVal, err := p.query(`rate(process_cpu_seconds_total[5m]) * 100`)
	if err == nil {
		m.CPUPercent = cpuVal
	}

	// Memory: resident memory bytes, convert to percentage (assume 8GB baseline)
	memVal, err := p.query(`process_resident_memory_bytes`)
	if err == nil {
		// Convert bytes to percentage of 8GB
		const baselineBytes = 8 * 1024 * 1024 * 1024
		m.MemoryPercent = (memVal / baselineBytes) * 100
	}

	// RPS: total request rate over 5 minutes
	rpsVal, err := p.query(`sum(rate(http_requests_total[5m]))`)
	if err == nil {
		m.RPS = rpsVal
	}

	// Error rate: 5xx rate / total rate as percentage
	errVal, err := p.query(`sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100`)
	if err == nil {
		m.ErrorRate = errVal
	}

	return m, nil
}

// query executes a single PromQL instant query and returns the scalar result.
func (p *PrometheusSource) query(promql string) (float64, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", p.Endpoint, url.QueryEscape(promql))
	resp, err := p.client.Get(u)
	if err != nil {
		return 0, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("reading prometheus response: %w", err)
	}

	var result promQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parsing prometheus response: %w", err)
	}

	if result.Status != "success" || len(result.Data.Result) == 0 {
		return 0, fmt.Errorf("no data for query: %s", promql)
	}

	// Value is [timestamp, "string_value"]
	valStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value type in prometheus response")
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing prometheus value %q: %w", valStr, err)
	}

	return val, nil
}

// HealthProbeCollector collects metrics from a JSON management endpoint.
type HealthProbeCollector struct {
	URL    string
	client *http.Client
}

// NewHealthProbeCollector creates a health-probe based collector.
func NewHealthProbeCollector(metricsURL string) *HealthProbeCollector {
	return &HealthProbeCollector{
		URL: metricsURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Collect fetches metrics from the management endpoint.
func (h *HealthProbeCollector) Collect() (*SystemMetrics, error) {
	resp, err := h.client.Get(h.URL)
	if err != nil {
		return nil, fmt.Errorf("health probe failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading health response: %w", err)
	}

	var m SystemMetrics
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parsing metrics: %w", err)
	}
	m.CollectedAt = time.Now()
	return &m, nil
}

// ApplyToCurrentMetrics merges system-level metrics into the observation-derived CurrentMetrics.
func ApplyToCurrentMetrics(sys *SystemMetrics, cm *objectives.CurrentMetrics) {
	if sys == nil || cm == nil {
		return
	}
	cm.CPUUsage = sys.CPUPercent
	cm.MemoryUsage = sys.MemoryPercent
	cm.Instances = int(sys.Instances)
	if sys.RPS > 0 {
		cm.CurrentRPS = int(sys.RPS)
	}
}
