// Package bridge provides integration between Stampede and a Stockyard proxy instance.
//
// When configured, Stampede routes synthetic user traffic through the Stockyard proxy
// instead of directly to the target. This gives conversations access to Stockyard's
// full middleware chain: cost tracking, guardrail testing, content filter verification,
// caching behavior, and rate limit testing.
//
// After a simulation completes, the bridge queries the proxy's trace API to enrich
// conversation results with middleware execution data.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds bridge configuration.
type Config struct {
	ProxyURL string // Base URL of the Stockyard proxy (e.g. http://localhost:8080)
	AdminKey string // Optional admin API key for trace access
	Timeout  time.Duration
}

// Bridge connects Stampede to a Stockyard proxy instance.
type Bridge struct {
	cfg    Config
	client *http.Client
}

// New creates a new Stockyard bridge.
func New(cfg Config) *Bridge {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Bridge{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// ProxyURL returns the configured proxy URL.
func (b *Bridge) ProxyURL() string { return b.cfg.ProxyURL }

// HealthCheck verifies the proxy is reachable.
func (b *Bridge) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", b.cfg.ProxyURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("proxy unreachable at %s: %w", b.cfg.ProxyURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("proxy returned %d", resp.StatusCode)
	}
	return nil
}

// TraceEnrichment holds data enriched from proxy traces.
type TraceEnrichment struct {
	ConversationID    string            `json:"conversation_id"`
	TotalTraces       int               `json:"total_traces"`
	CacheHits         int               `json:"cache_hits"`
	CacheMisses       int               `json:"cache_misses"`
	GuardrailTriggers int               `json:"guardrail_triggers"`
	ContentFilters    int               `json:"content_filter_triggers"`
	RateLimitHits     int               `json:"rate_limit_hits"`
	AvgLatencyMs      float64           `json:"avg_latency_ms"`
	TotalCostCents    float64           `json:"total_cost_cents"`
	ModulesExecuted   []string          `json:"modules_executed"`
	ProviderErrors    int               `json:"provider_errors"`
	Models            []string          `json:"models"`
	Metadata          map[string]string `json:"metadata"`
}

// SimulationEnrichment holds aggregate enrichment for a full simulation.
type SimulationEnrichment struct {
	SimulationID    string            `json:"simulation_id"`
	TotalTraces     int               `json:"total_traces"`
	CacheHitRate    float64           `json:"cache_hit_rate"`
	GuardrailTotal  int               `json:"guardrail_triggers_total"`
	RateLimitTotal  int               `json:"rate_limit_hits_total"`
	AvgLatencyMs    float64           `json:"avg_latency_ms"`
	TotalCostCents  float64           `json:"total_cost_cents"`
	UniqueModules   []string          `json:"unique_modules"`
	UniqueModels    []string          `json:"unique_models"`
	ProviderErrors  int               `json:"provider_errors_total"`
	Conversations   []TraceEnrichment `json:"conversations"`
}

// EnrichSimulation queries the Stockyard proxy for traces matching a simulation
// and returns enriched data.
func (b *Bridge) EnrichSimulation(ctx context.Context, simID string, conversationIDs []string) (*SimulationEnrichment, error) {
	enrichment := &SimulationEnrichment{
		SimulationID: simID,
	}

	// Query traces from the proxy's API
	// Stampede tags all requests with X-Stampede-Sim header, so we can filter by simulation
	traces, err := b.fetchTraces(ctx, simID)
	if err != nil {
		return nil, fmt.Errorf("fetching traces: %w", err)
	}

	// Group traces by conversation
	convTraces := map[string][]proxyTrace{}
	for _, t := range traces {
		cid := t.Tags["stampede_conversation"]
		if cid != "" {
			convTraces[cid] = append(convTraces[cid], t)
		}
	}

	// Build per-conversation enrichments
	modulesSet := map[string]bool{}
	modelsSet := map[string]bool{}
	var totalLatency float64
	traceCount := 0

	for _, cid := range conversationIDs {
		ct := convTraces[cid]
		te := TraceEnrichment{
			ConversationID: cid,
			TotalTraces:    len(ct),
			Metadata:       map[string]string{},
		}

		var convLatency float64
		for _, t := range ct {
			traceCount++
			convLatency += t.LatencyMs
			totalLatency += t.LatencyMs
			te.TotalCostCents += t.CostCents

			if t.CacheHit {
				te.CacheHits++
			} else {
				te.CacheMisses++
			}

			if t.GuardrailTriggered {
				te.GuardrailTriggers++
				enrichment.GuardrailTotal++
			}
			if t.ContentFiltered {
				te.ContentFilters++
			}
			if t.RateLimited {
				te.RateLimitHits++
				enrichment.RateLimitTotal++
			}
			if t.Error != "" {
				te.ProviderErrors++
				enrichment.ProviderErrors++
			}

			for _, m := range t.Modules {
				modulesSet[m] = true
			}
			if t.Model != "" {
				modelsSet[t.Model] = true
			}
		}

		if len(ct) > 0 {
			te.AvgLatencyMs = convLatency / float64(len(ct))
		}

		for m := range modulesSet {
			te.ModulesExecuted = append(te.ModulesExecuted, m)
		}
		for m := range modelsSet {
			te.Models = append(te.Models, m)
		}

		enrichment.Conversations = append(enrichment.Conversations, te)
		enrichment.TotalTraces += te.TotalTraces
		enrichment.TotalCostCents += te.TotalCostCents
	}

	if traceCount > 0 {
		enrichment.AvgLatencyMs = totalLatency / float64(traceCount)
	}

	totalCacheOps := 0
	totalCacheHits := 0
	for _, c := range enrichment.Conversations {
		totalCacheHits += c.CacheHits
		totalCacheOps += c.CacheHits + c.CacheMisses
	}
	if totalCacheOps > 0 {
		enrichment.CacheHitRate = float64(totalCacheHits) / float64(totalCacheOps) * 100
	}

	for m := range modulesSet {
		enrichment.UniqueModules = append(enrichment.UniqueModules, m)
	}
	for m := range modelsSet {
		enrichment.UniqueModels = append(enrichment.UniqueModels, m)
	}

	return enrichment, nil
}

// =========================================================================
// Proxy trace API types and fetching
// =========================================================================

type proxyTrace struct {
	ID                 string            `json:"id"`
	Model              string            `json:"model"`
	LatencyMs          float64           `json:"latency_ms"`
	CostCents          float64           `json:"cost_cents"`
	CacheHit           bool              `json:"cache_hit"`
	GuardrailTriggered bool              `json:"guardrail_triggered"`
	ContentFiltered    bool              `json:"content_filtered"`
	RateLimited        bool              `json:"rate_limited"`
	Error              string            `json:"error"`
	Modules            []string          `json:"modules_executed"`
	Tags               map[string]string `json:"tags"`
}

func (b *Bridge) fetchTraces(ctx context.Context, simID string) ([]proxyTrace, error) {
	url := fmt.Sprintf("%s/api/traces?tag=stampede_sim:%s&limit=10000", b.cfg.ProxyURL, simID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if b.cfg.AdminKey != "" {
		req.Header.Set("X-Admin-Key", b.cfg.AdminKey)
	}
	req.Header.Set("X-Stampede", "true")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying proxy traces: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("proxy returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		Traces []proxyTrace `json:"traces"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// Try as direct array
		var traces []proxyTrace
		if err2 := json.Unmarshal(body, &traces); err2 != nil {
			return nil, fmt.Errorf("parsing traces: %w", err)
		}
		return traces, nil
	}
	return result.Traces, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
