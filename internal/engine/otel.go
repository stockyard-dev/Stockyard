package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// OTELConfig holds OpenTelemetry export configuration.
type OTELConfig struct {
	Endpoint    string // OTEL collector endpoint (e.g., http://localhost:4318/v1/traces)
	ServiceName string // Service name for traces (default: "stockyard")
	Headers     map[string]string // Extra headers (e.g., auth tokens)
	BatchSize   int    // Spans per batch (default: 100)
	FlushInterval time.Duration // Flush interval (default: 5s)
}

// OTELExporter batches and exports spans to an OTLP-compatible endpoint.
type OTELExporter struct {
	cfg    OTELConfig
	client *http.Client
	mu     sync.Mutex
	batch  []otelSpan
	done   chan struct{}
}

type otelSpan struct {
	TraceID    string            `json:"traceId"`
	SpanID     string            `json:"spanId"`
	Name       string            `json:"name"`
	Kind       int               `json:"kind"` // 3 = CLIENT
	StartTime  int64             `json:"startTimeUnixNano"`
	EndTime    int64             `json:"endTimeUnixNano"`
	Attributes []otelAttribute   `json:"attributes"`
	Status     otelStatus        `json:"status"`
}

type otelAttribute struct {
	Key   string    `json:"key"`
	Value otelValue `json:"value"`
}

type otelValue struct {
	StringValue string `json:"stringValue,omitempty"`
	IntValue    int64  `json:"intValue,omitempty"`
	DoubleValue float64 `json:"doubleValue,omitempty"`
}

type otelStatus struct {
	Code    int    `json:"code"` // 0=unset, 1=ok, 2=error
	Message string `json:"message,omitempty"`
}

// LoadOTELConfig reads OTEL configuration from environment variables.
func LoadOTELConfig() *OTELConfig {
	endpoint := os.Getenv("STOCKYARD_OTEL_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		return nil // OTEL disabled
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "stockyard"
	}

	headers := make(map[string]string)
	if h := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); h != "" {
		// Parse key=value,key=value format
		for _, pair := range splitOTELHeaders(h) {
			if k, v, ok := splitKV(pair); ok {
				headers[k] = v
			}
		}
	}

	return &OTELConfig{
		Endpoint:      endpoint + "/v1/traces",
		ServiceName:   serviceName,
		Headers:       headers,
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
	}
}

// NewOTELExporter creates a new exporter (returns nil if OTEL is not configured).
func NewOTELExporter(cfg *OTELConfig) *OTELExporter {
	if cfg == nil {
		return nil
	}

	exp := &OTELExporter{
		cfg:    *cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		batch:  make([]otelSpan, 0, cfg.BatchSize),
		done:   make(chan struct{}),
	}

	go exp.flushLoop()
	log.Printf("[otel] exporting traces to %s (service: %s)", cfg.Endpoint, cfg.ServiceName)
	return exp
}

// OTELMiddleware creates a proxy middleware that exports spans.
func OTELMiddleware(exp *OTELExporter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			duration := time.Since(start)

			status := otelStatus{Code: 1} // OK
			statusStr := "ok"
			if err != nil {
				status = otelStatus{Code: 2, Message: err.Error()}
				statusStr = "error"
			}

			model := req.Model
			prov := ""
			tokens := 0
			cost := 0.0
			if resp != nil {
				prov = resp.Provider
				tokens = resp.Usage.TotalTokens
				model = resp.Model
			}

			span := otelSpan{
				TraceID:   fmt.Sprintf("%032x", time.Now().UnixNano()),
				SpanID:    fmt.Sprintf("%016x", time.Now().UnixNano()),
				Name:      "llm.chat.completion",
				Kind:      3,
				StartTime: start.UnixNano(),
				EndTime:   start.Add(duration).UnixNano(),
				Status:    status,
				Attributes: []otelAttribute{
					{Key: "llm.model", Value: otelValue{StringValue: model}},
					{Key: "llm.provider", Value: otelValue{StringValue: prov}},
					{Key: "llm.tokens.total", Value: otelValue{IntValue: int64(tokens)}},
					{Key: "llm.cost.usd", Value: otelValue{DoubleValue: cost}},
					{Key: "llm.latency.ms", Value: otelValue{DoubleValue: float64(duration.Milliseconds())}},
					{Key: "llm.status", Value: otelValue{StringValue: statusStr}},
					{Key: "service.name", Value: otelValue{StringValue: exp.cfg.ServiceName}},
				},
			}

			exp.addSpan(span)
			return resp, err
		}
	}
}

func (e *OTELExporter) addSpan(span otelSpan) {
	e.mu.Lock()
	e.batch = append(e.batch, span)
	shouldFlush := len(e.batch) >= e.cfg.BatchSize
	e.mu.Unlock()

	if shouldFlush {
		go e.flush()
	}
}

func (e *OTELExporter) flushLoop() {
	ticker := time.NewTicker(e.cfg.FlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.flush()
		case <-e.done:
			e.flush() // Final flush
			return
		}
	}
}

func (e *OTELExporter) flush() {
	e.mu.Lock()
	if len(e.batch) == 0 {
		e.mu.Unlock()
		return
	}
	spans := e.batch
	e.batch = make([]otelSpan, 0, e.cfg.BatchSize)
	e.mu.Unlock()

	// Build OTLP JSON payload
	payload := map[string]any{
		"resourceSpans": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []otelAttribute{
						{Key: "service.name", Value: otelValue{StringValue: e.cfg.ServiceName}},
					},
				},
				"scopeSpans": []map[string]any{
					{
						"scope": map[string]string{"name": "stockyard"},
						"spans": spans,
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[otel] marshal: %v", err)
		return
	}

	req, err := http.NewRequest("POST", e.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("[otel] request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		log.Printf("[otel] export failed: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[otel] export returned %d", resp.StatusCode)
	}
}

// Close flushes remaining spans and stops the exporter.
func (e *OTELExporter) Close() {
	close(e.done)
}

func splitOTELHeaders(s string) []string {
	parts := []string{}
	current := ""
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func splitKV(s string) (string, string, bool) {
	for i, c := range s {
		if c == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}
