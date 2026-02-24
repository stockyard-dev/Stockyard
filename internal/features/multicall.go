package features

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// MultiCaller sends the same request to multiple models and selects the best response.
type MultiCaller struct {
	routes  []config.MultiCallRoute
	stats   sync.Map // route_name → *multiCallStats
	total   atomic.Int64
	matched atomic.Int64
}

type multiCallStats struct {
	mu       sync.Mutex
	winCount map[string]int // model → wins
	total    int
}

// MultiCallResult holds the response from one model in a multi-call.
type MultiCallResult struct {
	Model    string
	Provider string
	Response *provider.Response
	Latency  time.Duration
	Error    error
	Cost     float64
}

// NewMultiCaller creates a new multi-model caller.
func NewMultiCaller(cfg config.MultiCallConfig) *MultiCaller {
	mc := &MultiCaller{
		routes: cfg.Routes,
	}
	for _, r := range cfg.Routes {
		mc.stats.Store(r.Name, &multiCallStats{winCount: make(map[string]int)})
	}
	return mc
}

// findRoute returns the matching route for a request, or nil.
func (mc *MultiCaller) findRoute(req *provider.Request) *config.MultiCallRoute {
	for i := range mc.routes {
		r := &mc.routes[i]
		// Match by header hint X-MultiCall-Route
		if hint, ok := req.Extra["X-MultiCall-Route"]; ok {
			if fmt.Sprintf("%v", hint) == r.Name {
				return r
			}
		}
		// Match all requests if route name is "default"
		if r.Name == "default" {
			return r
		}
	}
	// If exactly one route, use it
	if len(mc.routes) == 1 {
		return &mc.routes[0]
	}
	return nil
}

// Call sends the request to multiple models in parallel and returns the best response.
func (mc *MultiCaller) Call(ctx context.Context, req *provider.Request, sender func(ctx context.Context, model string, r *provider.Request) (*provider.Response, error)) (*provider.Response, error) {
	mc.total.Add(1)

	route := mc.findRoute(req)
	if route == nil {
		// No matching route — pass through with original model
		return nil, nil // signal to middleware to use next handler
	}

	mc.matched.Add(1)
	models := route.Models
	if len(models) < 2 {
		return nil, nil
	}

	timeout := 30 * time.Second
	if route.Timeout.Duration > 0 {
		timeout = route.Timeout.Duration
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Fan out to all models
	results := make([]MultiCallResult, len(models))
	var wg sync.WaitGroup
	for i, model := range models {
		wg.Add(1)
		go func(idx int, m string) {
			defer wg.Done()
			start := time.Now()
			cloned := cloneRequest(req)
			cloned.Model = m
			resp, err := sender(callCtx, m, cloned)
			results[idx] = MultiCallResult{
				Model:    m,
				Response: resp,
				Latency:  time.Since(start),
				Error:    err,
			}
			if resp != nil {
				results[idx].Cost = provider.CalculateCost(m, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			}
		}(i, model)
	}
	wg.Wait()

	// Filter successful results
	var successful []MultiCallResult
	for _, r := range results {
		if r.Error == nil && r.Response != nil && len(r.Response.Choices) > 0 {
			successful = append(successful, r)
		}
	}

	if len(successful) == 0 {
		// All failed — return the first error
		for _, r := range results {
			if r.Error != nil {
				return nil, r.Error
			}
		}
		return nil, fmt.Errorf("multicall: all %d models failed", len(models))
	}

	// Select winner based on strategy
	winner := mc.selectWinner(successful, route.Strategy)

	// Record stats
	if v, ok := mc.stats.Load(route.Name); ok {
		st := v.(*multiCallStats)
		st.mu.Lock()
		st.winCount[winner.Model]++
		st.total++
		st.mu.Unlock()
	}

	log.Printf("multicall: route=%s strategy=%s winner=%s latency=%v models_tried=%d/%d",
		route.Name, route.Strategy, winner.Model, winner.Latency, len(successful), len(models))

	return winner.Response, nil
}

// selectWinner picks the best response based on the strategy.
func (mc *MultiCaller) selectWinner(results []MultiCallResult, strategy string) MultiCallResult {
	switch strategy {
	case "fastest":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Latency < results[j].Latency
		})
		return results[0]

	case "cheapest", "cheapest_passing":
		sort.Slice(results, func(i, j int) bool {
			return results[i].Cost < results[j].Cost
		})
		return results[0]

	case "longest":
		sort.Slice(results, func(i, j int) bool {
			ai := contentLength(results[i].Response)
			aj := contentLength(results[j].Response)
			return ai > aj
		})
		return results[0]

	case "shortest":
		sort.Slice(results, func(i, j int) bool {
			ai := contentLength(results[i].Response)
			aj := contentLength(results[j].Response)
			return ai < aj
		})
		return results[0]

	case "consensus":
		// Find the response that most others agree with (by content similarity)
		if len(results) <= 2 {
			return results[0]
		}
		return mc.findConsensus(results)

	default:
		// Default: return the first successful result
		return results[0]
	}
}

// findConsensus selects the response most similar to others.
func (mc *MultiCaller) findConsensus(results []MultiCallResult) MultiCallResult {
	// Simple consensus: compare first 200 chars of each response
	contents := make([]string, len(results))
	for i, r := range results {
		c := ""
		if len(r.Response.Choices) > 0 {
			c = r.Response.Choices[0].Message.Content
		}
		if len(c) > 200 {
			c = c[:200]
		}
		contents[i] = strings.ToLower(c)
	}

	// Score each by similarity to others
	bestIdx := 0
	bestScore := -1.0
	for i := range contents {
		score := 0.0
		for j := range contents {
			if i != j {
				score += jaccardSimilarity(contents[i], contents[j])
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return results[bestIdx]
}

// jaccardSimilarity computes word-level Jaccard similarity between two strings.
func jaccardSimilarity(a, b string) float64 {
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)
	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 1.0
	}

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}
	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

func contentLength(resp *provider.Response) int {
	if resp == nil || len(resp.Choices) == 0 {
		return 0
	}
	return len(resp.Choices[0].Message.Content)
}

func cloneRequest(req *provider.Request) *provider.Request {
	clone := *req
	clone.Messages = make([]provider.Message, len(req.Messages))
	copy(clone.Messages, req.Messages)
	clone.Extra = make(map[string]any)
	for k, v := range req.Extra {
		clone.Extra[k] = v
	}
	return &clone
}

// Stats returns route stats.
func (mc *MultiCaller) Stats() map[string]any {
	stats := map[string]any{
		"total_requests": mc.total.Load(),
		"matched":        mc.matched.Load(),
	}
	routes := make(map[string]any)
	mc.stats.Range(func(key, value any) bool {
		name := key.(string)
		st := value.(*multiCallStats)
		st.mu.Lock()
		defer st.mu.Unlock()
		routes[name] = map[string]any{
			"total":     st.total,
			"win_count": st.winCount,
		}
		return true
	})
	stats["routes"] = routes
	return stats
}

// MultiCallMiddleware returns middleware that fans out to multiple models.
func MultiCallMiddleware(mc *MultiCaller, providers map[string]provider.Provider) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			sender := func(ctx context.Context, model string, r *provider.Request) (*provider.Response, error) {
				pName := provider.ProviderForModel(model)
				if r.Provider != "" {
					pName = r.Provider
				}
				p, ok := providers[pName]
				if !ok {
					return nil, fmt.Errorf("multicall: provider %s not available", pName)
				}
				return p.Send(ctx, r)
			}

			resp, err := mc.Call(ctx, req, sender)
			if resp != nil {
				return resp, err
			}
			if err != nil {
				return nil, err
			}
			// No route matched — pass through
			return next(ctx, req)
		}
	}
}
