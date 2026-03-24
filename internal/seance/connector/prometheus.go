// Prometheus metrics connector for Seance.
package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PrometheusSourceConfig holds configuration for a Prometheus data source.
type PrometheusSourceConfig struct {
	Endpoint string            `json:"endpoint"` // Prometheus base URL
	Queries  map[string]string `json:"queries"`  // name -> PromQL query
	Headers  map[string]string `json:"headers"`  // optional auth headers
}

// PrometheusSource fetches metrics from a Prometheus HTTP API.
type PrometheusSource struct {
	name   string
	config PrometheusSourceConfig
	client *http.Client
}

// NewPrometheusSource creates a Prometheus metrics connector.
func NewPrometheusSource(name string, config PrometheusSourceConfig) *PrometheusSource {
	if config.Queries == nil {
		config.Queries = defaultPromQueries()
	}
	return &PrometheusSource{
		name:   name,
		config: config,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *PrometheusSource) Name() string { return s.name }
func (s *PrometheusSource) Type() string { return "prometheus" }

func (s *PrometheusSource) Fetch(ctx context.Context, hints []string) (*Snapshot, error) {
	start := time.Now()
	snap := &Snapshot{
		Source:    s.name,
		Type:     "prometheus",
		FetchedAt: time.Now(),
	}

	var sb strings.Builder
	raw := map[string]any{}
	queryErrors := 0

	for qname, promql := range s.config.Queries {
		result, err := s.queryPrometheus(ctx, promql)
		if err != nil {
			sb.WriteString(fmt.Sprintf("--- %s ---\nERROR: %v\n\n", qname, err))
			raw[qname] = map[string]any{"error": err.Error()}
			queryErrors++
			continue
		}

		formatted := s.formatResult(qname, promql, result)
		sb.WriteString(formatted)
		sb.WriteString("\n")
		raw[qname] = result
	}

	snap.Data = sb.String()
	snap.Raw = raw
	snap.LatencyMs = int(time.Since(start).Milliseconds())

	if queryErrors > 0 && queryErrors == len(s.config.Queries) {
		snap.Error = "all Prometheus queries failed"
	}

	return snap, nil
}

// promResponse matches the Prometheus query API response shape.
type promResponse struct {
	Status string   `json:"status"`
	Data   promData `json:"data"`
	Error  string   `json:"error,omitempty"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promResult `json:"result"`
}

type promResult struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"` // [timestamp, string-value]
	Values [][]any           `json:"values"` // for range queries
}

func (s *PrometheusSource) queryPrometheus(ctx context.Context, promql string) (*promResponse, error) {
	endpoint := strings.TrimRight(s.config.Endpoint, "/") + "/api/v1/query"

	params := url.Values{}
	params.Set("query", promql)
	params.Set("time", fmt.Sprintf("%d", time.Now().Unix()))

	reqURL := endpoint + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range s.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}

	var promResp promResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", promResp.Error)
	}

	return &promResp, nil
}

func (s *PrometheusSource) formatResult(name, query string, resp *promResponse) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- %s ---\n", name))
	sb.WriteString(fmt.Sprintf("Query: %s\n", query))

	if len(resp.Data.Result) == 0 {
		sb.WriteString("(no data)\n")
		return sb.String()
	}

	for _, r := range resp.Data.Result {
		// Format metric labels
		var labels []string
		for k, v := range r.Metric {
			if k != "__name__" {
				labels = append(labels, fmt.Sprintf("%s=%q", k, v))
			}
		}

		metricName := r.Metric["__name__"]
		if metricName == "" {
			metricName = "result"
		}

		labelStr := ""
		if len(labels) > 0 {
			labelStr = "{" + strings.Join(labels, ", ") + "}"
		}

		// Format value
		if len(r.Value) >= 2 {
			val := fmt.Sprintf("%v", r.Value[1])
			sb.WriteString(fmt.Sprintf("  %s%s: %s\n", metricName, labelStr, formatMetricValue(val)))
		}

		// Format range values if present
		if len(r.Values) > 0 {
			sb.WriteString(fmt.Sprintf("  %s%s (range, %d points):\n", metricName, labelStr, len(r.Values)))
			// Show last 5 data points
			startIdx := 0
			if len(r.Values) > 5 {
				startIdx = len(r.Values) - 5
				sb.WriteString(fmt.Sprintf("    ... (%d earlier points)\n", startIdx))
			}
			for _, v := range r.Values[startIdx:] {
				if len(v) >= 2 {
					ts := fmt.Sprintf("%v", v[0])
					val := fmt.Sprintf("%v", v[1])
					sb.WriteString(fmt.Sprintf("    [%s] %s\n", ts, formatMetricValue(val)))
				}
			}
		}
	}

	return sb.String()
}

func formatMetricValue(val string) string {
	// Make common metric values more readable
	if val == "NaN" || val == "+Inf" || val == "-Inf" {
		return val
	}
	return val
}

func defaultPromQueries() map[string]string {
	return map[string]string{
		"error_rate":    `sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100`,
		"p99_latency":  `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))`,
		"request_rate": `sum(rate(http_requests_total[5m]))`,
		"memory_usage": `process_resident_memory_bytes / 1024 / 1024`,
	}
}
