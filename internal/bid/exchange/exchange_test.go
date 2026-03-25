package exchange

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// --- mock bidder ---

type mockBidder struct {
	id  string
	bid *Bid
	err error
	exec func(ctx context.Context, req *Request) (*CompletedRequest, error)
}

func (m *mockBidder) ID() string { return m.id }

func (m *mockBidder) SubmitBid(_ context.Context, _ *Request) (*Bid, error) {
	return m.bid, m.err
}

func (m *mockBidder) Execute(ctx context.Context, req *Request) (*CompletedRequest, error) {
	if m.exec != nil {
		return m.exec(ctx, req)
	}
	return &CompletedRequest{
		RequestID:  req.ID,
		ProviderID: m.id,
		Success:    true,
	}, nil
}

// --- mock store ---

type mockStore struct {
	saved   []AuctionResult
	listed  []AuctionResult
	listErr error
}

func (m *mockStore) SaveAuction(r AuctionResult) error {
	m.saved = append(m.saved, r)
	return nil
}

func (m *mockStore) ListAuctions(limit int) ([]AuctionResult, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if limit > len(m.listed) {
		return m.listed, nil
	}
	return m.listed[:limit], nil
}

// --- helpers ---

func makeRequest(id string) *Request {
	return &Request{
		ID:         id,
		Task:       "test",
		Prompt:     "hello",
		ReceivedAt: time.Now(),
		Requirements: Requirements{
			MaxPricePer1KTokens: 10.0,
			MaxLatencyMs:        5000,
			MinQualityScore:     0.1,
			BidTimeoutMs:        500,
		},
	}
}

func makeBidder(id string, price float64, latency int, quality float64) *mockBidder {
	return &mockBidder{
		id: id,
		bid: &Bid{
			ProviderID:   id,
			Model:        "test-model",
			PricePer1K:   price,
			EstLatencyMs: latency,
			QualityScore: quality,
		},
	}
}

// --- tests ---

func TestNewEngine_DefaultStrategy(t *testing.T) {
	e := New("")
	if e.strategy != StrategyCheapest {
		t.Errorf("expected default strategy %q, got %q", StrategyCheapest, e.strategy)
	}
}

func TestNewEngine_WithStrategy(t *testing.T) {
	for _, s := range []Strategy{StrategyCheapest, StrategyFastest, StrategyBestQuality, StrategyBalanced} {
		e := New(s)
		if e.strategy != s {
			t.Errorf("expected strategy %q, got %q", s, e.strategy)
		}
	}
}

func TestRegisterUnregisterBidder(t *testing.T) {
	e := New(StrategyCheapest)
	b := &mockBidder{id: "p1"}

	e.RegisterBidder(b)
	if e.BidderCount() != 1 {
		t.Fatalf("expected 1 bidder, got %d", e.BidderCount())
	}

	e.UnregisterBidder("p1")
	if e.BidderCount() != 0 {
		t.Fatalf("expected 0 bidders, got %d", e.BidderCount())
	}
}

func TestFilterEligible(t *testing.T) {
	tests := []struct {
		name     string
		bids     []Bid
		reqs     Requirements
		wantLen  int
	}{
		{
			name: "all eligible",
			bids: []Bid{
				{PricePer1K: 1.0, EstLatencyMs: 100, QualityScore: 0.9},
				{PricePer1K: 2.0, EstLatencyMs: 200, QualityScore: 0.8},
			},
			reqs:    Requirements{MaxPricePer1KTokens: 5.0, MaxLatencyMs: 500, MinQualityScore: 0.5},
			wantLen: 2,
		},
		{
			name: "filter by price",
			bids: []Bid{
				{PricePer1K: 1.0, EstLatencyMs: 100, QualityScore: 0.9},
				{PricePer1K: 10.0, EstLatencyMs: 100, QualityScore: 0.9},
			},
			reqs:    Requirements{MaxPricePer1KTokens: 5.0},
			wantLen: 1,
		},
		{
			name: "filter by latency",
			bids: []Bid{
				{PricePer1K: 1.0, EstLatencyMs: 100, QualityScore: 0.9},
				{PricePer1K: 1.0, EstLatencyMs: 9000, QualityScore: 0.9},
			},
			reqs:    Requirements{MaxLatencyMs: 500},
			wantLen: 1,
		},
		{
			name: "filter by quality",
			bids: []Bid{
				{PricePer1K: 1.0, EstLatencyMs: 100, QualityScore: 0.9},
				{PricePer1K: 1.0, EstLatencyMs: 100, QualityScore: 0.1},
			},
			reqs:    Requirements{MinQualityScore: 0.5},
			wantLen: 1,
		},
		{
			name: "none eligible",
			bids: []Bid{
				{PricePer1K: 100.0, EstLatencyMs: 9999, QualityScore: 0.01},
			},
			reqs:    Requirements{MaxPricePer1KTokens: 5.0, MaxLatencyMs: 500, MinQualityScore: 0.5},
			wantLen: 0,
		},
		{
			name:    "zero requirements pass everything",
			bids:    []Bid{{PricePer1K: 100, EstLatencyMs: 9999, QualityScore: 0.01}},
			reqs:    Requirements{},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEligible(tt.bids, tt.reqs)
			if len(got) != tt.wantLen {
				t.Errorf("filterEligible returned %d bids, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSelectWinner_Cheapest(t *testing.T) {
	e := New(StrategyCheapest)
	bids := []Bid{
		{ProviderID: "expensive", PricePer1K: 5.0, EstLatencyMs: 100, QualityScore: 0.9},
		{ProviderID: "cheap", PricePer1K: 0.5, EstLatencyMs: 500, QualityScore: 0.5},
		{ProviderID: "mid", PricePer1K: 2.0, EstLatencyMs: 200, QualityScore: 0.7},
	}
	winner, _ := e.selectWinner(bids, Requirements{})
	if winner.ProviderID != "cheap" {
		t.Errorf("expected cheapest provider, got %s", winner.ProviderID)
	}
}

func TestSelectWinner_Fastest(t *testing.T) {
	e := New(StrategyFastest)
	bids := []Bid{
		{ProviderID: "slow", PricePer1K: 0.5, EstLatencyMs: 500, QualityScore: 0.9},
		{ProviderID: "fast", PricePer1K: 5.0, EstLatencyMs: 50, QualityScore: 0.5},
		{ProviderID: "mid", PricePer1K: 2.0, EstLatencyMs: 200, QualityScore: 0.7},
	}
	winner, _ := e.selectWinner(bids, Requirements{})
	if winner.ProviderID != "fast" {
		t.Errorf("expected fastest provider, got %s", winner.ProviderID)
	}
}

func TestSelectWinner_BestQuality(t *testing.T) {
	e := New(StrategyBestQuality)
	bids := []Bid{
		{ProviderID: "low", PricePer1K: 0.5, EstLatencyMs: 50, QualityScore: 0.3},
		{ProviderID: "high", PricePer1K: 5.0, EstLatencyMs: 500, QualityScore: 0.99},
		{ProviderID: "mid", PricePer1K: 2.0, EstLatencyMs: 200, QualityScore: 0.7},
	}
	winner, _ := e.selectWinner(bids, Requirements{})
	if winner.ProviderID != "high" {
		t.Errorf("expected highest quality provider, got %s", winner.ProviderID)
	}
}

func TestSelectWinner_Balanced(t *testing.T) {
	e := New(StrategyBalanced)
	// The balanced provider is cheapest AND fastest AND highest quality — should win.
	bids := []Bid{
		{ProviderID: "balanced", PricePer1K: 1.0, EstLatencyMs: 100, QualityScore: 0.9},
		{ProviderID: "bad", PricePer1K: 10.0, EstLatencyMs: 1000, QualityScore: 0.1},
	}
	winner, reason := e.selectWinner(bids, Requirements{})
	if winner.ProviderID != "balanced" {
		t.Errorf("expected balanced provider, got %s (reason: %s)", winner.ProviderID, reason)
	}
}

func TestSelectBalanced_WeightsCorrectly(t *testing.T) {
	// Two bids where one is cheaper but lower quality; the other is expensive but high quality.
	// With weights 40% price, 25% latency, 35% quality, quality-heavy bid can win
	// if quality advantage outweighs price disadvantage.
	bids := []Bid{
		{ProviderID: "cheap-low-q", PricePer1K: 1.0, EstLatencyMs: 200, QualityScore: 0.2},
		{ProviderID: "expensive-high-q", PricePer1K: 5.0, EstLatencyMs: 200, QualityScore: 1.0},
	}
	winner, _ := selectBalanced(bids, Requirements{})
	// cheap-low-q: price=1-(1/5)=0.8, latency=0, quality=0.2 => 0.8*0.4+0*0.25+0.2*0.35=0.39
	// expensive-high-q: price=1-(5/5)=0, latency=0, quality=1.0 => 0*0.4+0*0.25+1.0*0.35=0.35
	if winner.ProviderID != "cheap-low-q" {
		t.Errorf("expected cheap-low-q to win with balanced scoring, got %s", winner.ProviderID)
	}
}

func TestAuction_NoBids(t *testing.T) {
	e := New(StrategyCheapest)
	// Register a bidder that returns nil
	e.RegisterBidder(&mockBidder{id: "nil-bidder", bid: nil})

	req := makeRequest("r1")
	result, completed, err := e.Auction(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for no bids")
	}
	if completed != nil {
		t.Fatal("expected nil completed when no bids")
	}
	if result.NoBidsReason == "" {
		t.Error("expected NoBidsReason to be set")
	}
}

func TestAuction_NoneEligible(t *testing.T) {
	e := New(StrategyCheapest)
	// Bid too expensive for the requirements
	e.RegisterBidder(makeBidder("pricey", 100.0, 100, 0.9))

	req := makeRequest("r2")
	req.Requirements.MaxPricePer1KTokens = 1.0

	_, _, err := e.Auction(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for no eligible bids")
	}
}

func TestAuction_Success(t *testing.T) {
	e := New(StrategyCheapest)
	e.RegisterBidder(makeBidder("p1", 1.0, 100, 0.9))
	e.RegisterBidder(makeBidder("p2", 2.0, 200, 0.8))

	req := makeRequest("r3")
	result, completed, err := e.Auction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Winner == nil {
		t.Fatal("expected a winner")
	}
	if result.Winner.ProviderID != "p1" {
		t.Errorf("expected p1 to win (cheapest), got %s", result.Winner.ProviderID)
	}
	if result.BidCount != 2 {
		t.Errorf("expected 2 bids, got %d", result.BidCount)
	}
	if completed == nil {
		t.Fatal("expected completed request")
	}
	if !completed.Success {
		t.Error("expected success")
	}
}

func TestAuction_BidderError(t *testing.T) {
	e := New(StrategyCheapest)
	// One bidder errors, the other succeeds
	e.RegisterBidder(&mockBidder{id: "failing", err: fmt.Errorf("network timeout")})
	e.RegisterBidder(makeBidder("working", 1.0, 100, 0.9))

	req := makeRequest("r4")
	result, _, err := e.Auction(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BidCount != 1 {
		t.Errorf("expected 1 bid (failing bidder excluded), got %d", result.BidCount)
	}
}

func TestRecentAuctions(t *testing.T) {
	e := New(StrategyCheapest)
	e.RegisterBidder(makeBidder("p1", 1.0, 100, 0.9))

	for i := 0; i < 5; i++ {
		req := makeRequest(fmt.Sprintf("r%d", i))
		e.Auction(context.Background(), req)
	}

	recent := e.RecentAuctions(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent auctions, got %d", len(recent))
	}

	all := e.RecentAuctions(100)
	if len(all) != 5 {
		t.Errorf("expected 5 total auctions, got %d", len(all))
	}
}

func TestStats(t *testing.T) {
	e := New(StrategyCheapest)
	e.RegisterBidder(makeBidder("p1", 1.0, 100, 0.9))
	e.RegisterBidder(makeBidder("p2", 2.0, 200, 0.8))

	for i := 0; i < 3; i++ {
		req := makeRequest(fmt.Sprintf("r%d", i))
		e.Auction(context.Background(), req)
	}

	stats := e.Stats()
	if stats.RegisteredProviders != 2 {
		t.Errorf("expected 2 providers, got %d", stats.RegisteredProviders)
	}
	if stats.TotalAuctions != 3 {
		t.Errorf("expected 3 auctions, got %d", stats.TotalAuctions)
	}
	if stats.AvgBidsPerAuction != 2.0 {
		t.Errorf("expected avg 2 bids, got %.1f", stats.AvgBidsPerAuction)
	}
	// p1 wins all 3 (cheapest)
	if stats.WinRateByProvider["p1"] != 100.0 {
		t.Errorf("expected p1 win rate 100%%, got %.1f%%", stats.WinRateByProvider["p1"])
	}
}

func TestEngine_WithStore(t *testing.T) {
	ms := &mockStore{}
	e := New(StrategyCheapest, ms)
	e.RegisterBidder(makeBidder("p1", 1.0, 100, 0.9))

	req := makeRequest("r1")
	e.Auction(context.Background(), req)

	if len(ms.saved) != 1 {
		t.Errorf("expected 1 saved auction, got %d", len(ms.saved))
	}
	if ms.saved[0].RequestID != "r1" {
		t.Errorf("expected request ID r1, got %s", ms.saved[0].RequestID)
	}
}
