// Package exchange implements the real-time auction engine where LLM providers
// compete for requests. Each incoming request is broadcast to registered providers,
// who submit bids (price + estimated latency + quality score). The exchange selects
// the winning bid, routes the request, and tracks delivered quality against promises.
package exchange

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Request represents an incoming LLM request to be auctioned.
type Request struct {
	ID           string         `json:"id"`
	Task         string         `json:"task"`          // classification, summarization, generation, etc.
	Prompt       string         `json:"prompt"`
	Messages     []Message      `json:"messages,omitempty"`
	Requirements Requirements   `json:"requirements"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	ReceivedAt   time.Time      `json:"received_at"`
}

// Message is a chat message (OpenAI-compatible).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Requirements define what the buyer needs from the winning provider.
type Requirements struct {
	MaxPricePer1KTokens float64 `json:"max_price_per_1k_tokens"` // max $/1K tokens
	MaxLatencyMs        int     `json:"max_latency_ms"`          // max acceptable latency
	MinQualityScore     float64 `json:"min_quality_score"`       // 0.0-1.0 minimum quality
	Format              string  `json:"format,omitempty"`        // json, text, etc.
	MaxTokens           int     `json:"max_tokens,omitempty"`
	BidTimeoutMs        int     `json:"bid_timeout_ms"`          // how long to wait for bids (default 100)
}

// Bid is a provider's offer to fulfill a request.
type Bid struct {
	ProviderID     string  `json:"provider_id"`
	Model          string  `json:"model"`
	PricePer1K     float64 `json:"price_per_1k_tokens"`
	EstLatencyMs   int     `json:"est_latency_ms"`
	QualityScore   float64 `json:"quality_score"`     // self-assessed 0.0-1.0
	ConfidenceScore float64 `json:"confidence_score"` // how confident in quality estimate
	ReceivedAt     time.Time `json:"received_at"`
}

// AuctionResult holds the outcome of a single auction.
type AuctionResult struct {
	RequestID    string    `json:"request_id"`
	Winner       *Bid      `json:"winner"`
	AllBids      []Bid     `json:"all_bids"`
	BidCount     int       `json:"bid_count"`
	AuctionTime  time.Duration `json:"auction_time_ms"`
	Reason       string    `json:"reason"` // why this bid won
	NoBidsReason string    `json:"no_bids_reason,omitempty"`
}

// CompletedRequest tracks what actually happened after the auction.
type CompletedRequest struct {
	RequestID      string        `json:"request_id"`
	ProviderID     string        `json:"provider_id"`
	Model          string        `json:"model"`
	BidPrice       float64       `json:"bid_price_per_1k"`
	ActualLatencyMs int          `json:"actual_latency_ms"`
	ActualTokens   int           `json:"actual_tokens"`
	ActualCost     float64       `json:"actual_cost_cents"`
	Response       string        `json:"response"`
	Success        bool          `json:"success"`
	Error          string        `json:"error,omitempty"`
	CompletedAt    time.Time     `json:"completed_at"`
}

// Strategy controls how the exchange selects winners.
type Strategy string

const (
	StrategyCheapest    Strategy = "cheapest"     // lowest price meeting all requirements
	StrategyFastest     Strategy = "fastest"       // lowest latency meeting all requirements
	StrategyBestQuality Strategy = "best_quality"  // highest quality meeting all requirements
	StrategyBalanced    Strategy = "balanced"       // weighted score across all dimensions
)

// Engine is the core auction engine.
type Engine struct {
	mu       sync.RWMutex
	bidders  map[string]Bidder // registered providers
	strategy Strategy
	history  []AuctionResult   // recent auctions for analytics
	maxHist  int
}

// Bidder interface — providers implement this to participate in auctions.
type Bidder interface {
	ID() string
	SubmitBid(ctx context.Context, req *Request) (*Bid, error)
	Execute(ctx context.Context, req *Request) (*CompletedRequest, error)
}

// New creates a new auction engine.
func New(strategy Strategy) *Engine {
	if strategy == "" {
		strategy = StrategyCheapest
	}
	return &Engine{
		bidders:  make(map[string]Bidder),
		strategy: strategy,
		maxHist:  10000,
	}
}

// RegisterBidder adds a provider to the exchange.
func (e *Engine) RegisterBidder(b Bidder) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bidders[b.ID()] = b
}

// UnregisterBidder removes a provider.
func (e *Engine) UnregisterBidder(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.bidders, id)
}

// BidderCount returns the number of registered providers.
func (e *Engine) BidderCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.bidders)
}

// Auction runs a complete auction: collect bids, select winner, execute, return result.
func (e *Engine) Auction(ctx context.Context, req *Request) (*AuctionResult, *CompletedRequest, error) {
	start := time.Now()

	// Collect bids from all registered providers
	bids := e.collectBids(ctx, req)

	result := &AuctionResult{
		RequestID:   req.ID,
		AllBids:     bids,
		BidCount:    len(bids),
		AuctionTime: time.Since(start),
	}

	if len(bids) == 0 {
		result.NoBidsReason = "no providers submitted bids within timeout"
		e.recordResult(result)
		return result, nil, fmt.Errorf("no bids received for request %s", req.ID)
	}

	// Filter bids that meet requirements
	eligible := filterEligible(bids, req.Requirements)
	if len(eligible) == 0 {
		result.NoBidsReason = fmt.Sprintf("%d bids received but none met requirements", len(bids))
		e.recordResult(result)
		return result, nil, fmt.Errorf("no eligible bids: %d bids received but none met requirements", len(bids))
	}

	// Select winner based on strategy
	winner, reason := e.selectWinner(eligible, req.Requirements)
	result.Winner = winner
	result.Reason = reason

	// Execute the request with the winner
	e.mu.RLock()
	bidder, ok := e.bidders[winner.ProviderID]
	e.mu.RUnlock()
	if !ok {
		return result, nil, fmt.Errorf("winning bidder %s no longer registered", winner.ProviderID)
	}

	completed, err := bidder.Execute(ctx, req)
	if err != nil {
		return result, nil, fmt.Errorf("execution by %s failed: %w", winner.ProviderID, err)
	}

	e.recordResult(result)
	return result, completed, nil
}

// collectBids broadcasts to all providers and collects bids within timeout.
func (e *Engine) collectBids(ctx context.Context, req *Request) []Bid {
	timeout := time.Duration(req.Requirements.BidTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}

	bidCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	e.mu.RLock()
	bidders := make([]Bidder, 0, len(e.bidders))
	for _, b := range e.bidders {
		bidders = append(bidders, b)
	}
	e.mu.RUnlock()

	var (
		bids []Bid
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	for _, b := range bidders {
		wg.Add(1)
		go func(b Bidder) {
			defer wg.Done()
			bid, err := b.SubmitBid(bidCtx, req)
			if err != nil || bid == nil {
				return
			}
			bid.ReceivedAt = time.Now()
			mu.Lock()
			bids = append(bids, *bid)
			mu.Unlock()
		}(b)
	}

	wg.Wait()
	return bids
}

// filterEligible removes bids that don't meet the buyer's requirements.
func filterEligible(bids []Bid, reqs Requirements) []Bid {
	var eligible []Bid
	for _, b := range bids {
		if reqs.MaxPricePer1KTokens > 0 && b.PricePer1K > reqs.MaxPricePer1KTokens {
			continue
		}
		if reqs.MaxLatencyMs > 0 && b.EstLatencyMs > reqs.MaxLatencyMs {
			continue
		}
		if reqs.MinQualityScore > 0 && b.QualityScore < reqs.MinQualityScore {
			continue
		}
		eligible = append(eligible, b)
	}
	return eligible
}

// selectWinner picks the best bid based on the exchange strategy.
func (e *Engine) selectWinner(bids []Bid, reqs Requirements) (*Bid, string) {
	switch e.strategy {
	case StrategyCheapest:
		sort.Slice(bids, func(i, j int) bool { return bids[i].PricePer1K < bids[j].PricePer1K })
		return &bids[0], fmt.Sprintf("cheapest at $%.4f/1K tokens", bids[0].PricePer1K)

	case StrategyFastest:
		sort.Slice(bids, func(i, j int) bool { return bids[i].EstLatencyMs < bids[j].EstLatencyMs })
		return &bids[0], fmt.Sprintf("fastest at %dms estimated latency", bids[0].EstLatencyMs)

	case StrategyBestQuality:
		sort.Slice(bids, func(i, j int) bool { return bids[i].QualityScore > bids[j].QualityScore })
		return &bids[0], fmt.Sprintf("highest quality at %.2f", bids[0].QualityScore)

	case StrategyBalanced:
		return selectBalanced(bids, reqs)

	default:
		sort.Slice(bids, func(i, j int) bool { return bids[i].PricePer1K < bids[j].PricePer1K })
		return &bids[0], "cheapest (default)"
	}
}

// selectBalanced uses a weighted scoring function across price, latency, and quality.
func selectBalanced(bids []Bid, reqs Requirements) (*Bid, string) {
	// Normalize each dimension to 0-1 range
	var maxPrice, maxLatency float64
	for _, b := range bids {
		if b.PricePer1K > maxPrice {
			maxPrice = b.PricePer1K
		}
		if float64(b.EstLatencyMs) > maxLatency {
			maxLatency = float64(b.EstLatencyMs)
		}
	}
	if maxPrice == 0 {
		maxPrice = 1
	}
	if maxLatency == 0 {
		maxLatency = 1
	}

	type scored struct {
		bid   Bid
		score float64
	}

	var candidates []scored
	for _, b := range bids {
		// Lower is better for price and latency; higher is better for quality
		priceScore := 1 - (b.PricePer1K / maxPrice)           // 0=expensive, 1=cheap
		latencyScore := 1 - (float64(b.EstLatencyMs) / maxLatency) // 0=slow, 1=fast
		qualityScore := b.QualityScore                          // 0=bad, 1=good

		// Weights: 40% price, 25% latency, 35% quality
		total := priceScore*0.40 + latencyScore*0.25 + qualityScore*0.35
		candidates = append(candidates, scored{bid: b, score: total})
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	winner := candidates[0]
	return &winner.bid, fmt.Sprintf("balanced score %.3f (price=%.4f, latency=%dms, quality=%.2f)",
		winner.score, winner.bid.PricePer1K, winner.bid.EstLatencyMs, winner.bid.QualityScore)
}

func (e *Engine) recordResult(r *AuctionResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = append(e.history, *r)
	if len(e.history) > e.maxHist {
		e.history = e.history[len(e.history)-e.maxHist:]
	}
}

// RecentAuctions returns the last N auction results.
func (e *Engine) RecentAuctions(n int) []AuctionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > len(e.history) {
		n = len(e.history)
	}
	out := make([]AuctionResult, n)
	copy(out, e.history[len(e.history)-n:])
	return out
}

// Stats returns aggregate exchange statistics.
type Stats struct {
	RegisteredProviders int     `json:"registered_providers"`
	TotalAuctions       int     `json:"total_auctions"`
	AvgBidsPerAuction   float64 `json:"avg_bids_per_auction"`
	AvgAuctionTimeMs    float64 `json:"avg_auction_time_ms"`
	WinRateByProvider   map[string]float64 `json:"win_rate_by_provider"`
	AvgSavingsVsDirect  float64 `json:"avg_savings_vs_direct_pct"`
}

func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s := Stats{
		RegisteredProviders: len(e.bidders),
		TotalAuctions:       len(e.history),
		WinRateByProvider:   map[string]float64{},
	}

	if len(e.history) == 0 {
		return s
	}

	var totalBids int
	var totalTime float64
	winCounts := map[string]int{}

	for _, a := range e.history {
		totalBids += a.BidCount
		totalTime += float64(a.AuctionTime.Milliseconds())
		if a.Winner != nil {
			winCounts[a.Winner.ProviderID]++
		}
	}

	s.AvgBidsPerAuction = float64(totalBids) / float64(len(e.history))
	s.AvgAuctionTimeMs = totalTime / float64(len(e.history))

	for provider, wins := range winCounts {
		s.WinRateByProvider[provider] = float64(wins) / float64(len(e.history)) * 100
	}

	return s
}
