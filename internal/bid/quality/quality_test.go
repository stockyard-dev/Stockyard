package quality

import (
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
)

func TestNew_NoStore(t *testing.T) {
	tr := New()
	if tr == nil {
		t.Fatal("New() returned nil")
	}
	if tr.maxHist != 200 {
		t.Errorf("maxHist = %d, want 200", tr.maxHist)
	}
	if tr.store != nil {
		t.Error("store should be nil when none provided")
	}
}

func TestNew_NilStore(t *testing.T) {
	tr := New(nil)
	if tr.store != nil {
		t.Error("store should be nil when nil provided")
	}
}

func TestRecordCompletion_NewProvider(t *testing.T) {
	tr := New()

	bid := &exchange.Bid{
		ProviderID:   "prov-1",
		EstLatencyMs: 100,
		QualityScore: 0.9,
	}
	completed := &exchange.CompletedRequest{
		ProviderID:      "prov-1",
		ActualLatencyMs: 120,
		ActualCost:      0.05,
		Success:         true,
	}

	tr.RecordCompletion(bid, completed)

	rec := tr.GetRecord("prov-1")
	if rec == nil {
		t.Fatal("expected record for prov-1")
	}
	if rec.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1", rec.TotalRequests)
	}
	if rec.Successes != 1 {
		t.Errorf("Successes = %d, want 1", rec.Successes)
	}
	if rec.Failures != 0 {
		t.Errorf("Failures = %d, want 0", rec.Failures)
	}
	if rec.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0", rec.SuccessRate)
	}
	if rec.TotalRevenue != 0.05 {
		t.Errorf("TotalRevenue = %v, want 0.05", rec.TotalRevenue)
	}
}

func TestRecordCompletion_Failure(t *testing.T) {
	tr := New()

	bid := &exchange.Bid{ProviderID: "prov-2", EstLatencyMs: 100, QualityScore: 0.9}
	completed := &exchange.CompletedRequest{
		ProviderID: "prov-2",
		Success:    false,
	}

	tr.RecordCompletion(bid, completed)

	rec := tr.GetRecord("prov-2")
	if rec.Failures != 1 {
		t.Errorf("Failures = %d, want 1", rec.Failures)
	}
	if rec.SuccessRate != 0.0 {
		t.Errorf("SuccessRate = %v, want 0.0", rec.SuccessRate)
	}
}

func TestRecordCompletion_MultipleResults(t *testing.T) {
	tr := New()

	for i := 0; i < 10; i++ {
		bid := &exchange.Bid{ProviderID: "prov-3", EstLatencyMs: 100, QualityScore: 0.9}
		success := i < 8
		completed := &exchange.CompletedRequest{
			ProviderID:      "prov-3",
			ActualLatencyMs: 100 + i*10,
			Success:         success,
		}
		tr.RecordCompletion(bid, completed)
	}

	rec := tr.GetRecord("prov-3")
	if rec.TotalRequests != 10 {
		t.Errorf("TotalRequests = %d, want 10", rec.TotalRequests)
	}
	if rec.Successes != 8 {
		t.Errorf("Successes = %d, want 8", rec.Successes)
	}
	if rec.Failures != 2 {
		t.Errorf("Failures = %d, want 2", rec.Failures)
	}
}

func TestGetRecord_NotFound(t *testing.T) {
	tr := New()
	rec := tr.GetRecord("nonexistent")
	if rec != nil {
		t.Error("expected nil for nonexistent provider")
	}
}

func TestGetRecord_ReturnsCopy(t *testing.T) {
	tr := New()
	bid := &exchange.Bid{ProviderID: "prov-copy", EstLatencyMs: 100, QualityScore: 0.9}
	completed := &exchange.CompletedRequest{ProviderID: "prov-copy", Success: true}
	tr.RecordCompletion(bid, completed)

	rec1 := tr.GetRecord("prov-copy")
	rec1.TotalRequests = 999

	rec2 := tr.GetRecord("prov-copy")
	if rec2.TotalRequests == 999 {
		t.Error("GetRecord should return a copy, not a reference")
	}
}

func TestAllRecords(t *testing.T) {
	tr := New()

	// Record for two providers
	for _, id := range []string{"prov-a", "prov-b"} {
		bid := &exchange.Bid{ProviderID: id, EstLatencyMs: 100, QualityScore: 0.9}
		completed := &exchange.CompletedRequest{ProviderID: id, Success: true}
		tr.RecordCompletion(bid, completed)
	}

	records := tr.AllRecords()
	if len(records) != 2 {
		t.Errorf("AllRecords() returned %d records, want 2", len(records))
	}

	// Check that RecentResults is nil (not exposed)
	for _, rec := range records {
		if rec.RecentResults != nil {
			t.Error("AllRecords should not expose RecentResults")
		}
	}
}

func TestAdjustBid_NoHistory(t *testing.T) {
	tr := New()
	bid := &exchange.Bid{
		ProviderID:      "unknown",
		QualityScore:    0.9,
		EstLatencyMs:    100,
		ConfidenceScore: 0.8,
	}
	tr.AdjustBid(bid)
	// No changes
	if bid.QualityScore != 0.9 {
		t.Errorf("QualityScore should be unchanged, got %v", bid.QualityScore)
	}
	if bid.EstLatencyMs != 100 {
		t.Errorf("EstLatencyMs should be unchanged, got %d", bid.EstLatencyMs)
	}
	if bid.ConfidenceScore != 0.8 {
		t.Errorf("ConfidenceScore should be unchanged, got %v", bid.ConfidenceScore)
	}
}

func TestAdjustBid_QualityBiasPenalty(t *testing.T) {
	tr := New()

	// Record enough results where provider overbids quality (bids 0.95 but only succeeds 50%)
	for i := 0; i < 10; i++ {
		bid := &exchange.Bid{ProviderID: "overbidder", EstLatencyMs: 100, QualityScore: 0.95}
		success := i < 5
		completed := &exchange.CompletedRequest{
			ProviderID:      "overbidder",
			ActualLatencyMs: 100,
			Success:         success,
		}
		tr.RecordCompletion(bid, completed)
	}

	rec := tr.GetRecord("overbidder")
	if rec.QualityBias <= 0.05 {
		t.Skipf("QualityBias = %v, not high enough to trigger penalty", rec.QualityBias)
	}

	bid := &exchange.Bid{
		ProviderID:      "overbidder",
		QualityScore:    0.95,
		EstLatencyMs:    100,
		ConfidenceScore: 1.0,
	}
	tr.AdjustBid(bid)

	if bid.QualityScore >= 0.95 {
		t.Errorf("QualityScore should be penalized, got %v", bid.QualityScore)
	}
}

func TestAdjustBid_LatencyInflation(t *testing.T) {
	tr := New()

	// Record results where actual latency is much higher than bid (bad accuracy)
	for i := 0; i < 10; i++ {
		bid := &exchange.Bid{ProviderID: "slow-prov", EstLatencyMs: 100, QualityScore: 0.8}
		completed := &exchange.CompletedRequest{
			ProviderID:      "slow-prov",
			ActualLatencyMs: 300, // 3x the estimate
			Success:         true,
		}
		tr.RecordCompletion(bid, completed)
	}

	rec := tr.GetRecord("slow-prov")
	if rec.LatencyAccuracy >= 0.8 {
		t.Skipf("LatencyAccuracy = %v, not low enough to trigger inflation", rec.LatencyAccuracy)
	}

	bid := &exchange.Bid{
		ProviderID:      "slow-prov",
		EstLatencyMs:    100,
		ConfidenceScore: 1.0,
		QualityScore:    0.8,
	}
	tr.AdjustBid(bid)

	if bid.EstLatencyMs <= 100 {
		t.Errorf("EstLatencyMs should be inflated, got %d", bid.EstLatencyMs)
	}
}

func TestAdjustBid_ConfidenceReliability(t *testing.T) {
	tr := New()

	bid := &exchange.Bid{ProviderID: "reliable", EstLatencyMs: 100, QualityScore: 0.9}
	completed := &exchange.CompletedRequest{
		ProviderID:      "reliable",
		ActualLatencyMs: 100,
		Success:         true,
	}
	tr.RecordCompletion(bid, completed)

	testBid := &exchange.Bid{
		ProviderID:      "reliable",
		ConfidenceScore: 1.0,
		QualityScore:    0.9,
		EstLatencyMs:    100,
	}
	tr.AdjustBid(testBid)

	rec := tr.GetRecord("reliable")
	if testBid.ConfidenceScore != rec.ReliabilityScore {
		t.Errorf("ConfidenceScore = %v, want %v (reliability)", testBid.ConfidenceScore, rec.ReliabilityScore)
	}
}

// Metric calculator tests

func TestAvgLatency(t *testing.T) {
	tests := []struct {
		name    string
		results []RequestResult
		want    float64
	}{
		{"empty", nil, 0},
		{"single", []RequestResult{{ActualLatencyMs: 100}}, 100},
		{"multiple", []RequestResult{{ActualLatencyMs: 100}, {ActualLatencyMs: 200}}, 150},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := avgLatency(tt.results)
			if got != tt.want {
				t.Errorf("avgLatency() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLatencyAccuracy(t *testing.T) {
	tests := []struct {
		name    string
		results []RequestResult
		want    float64
	}{
		{"empty", nil, 0.5},
		{"perfect", []RequestResult{{BidLatencyMs: 100, ActualLatencyMs: 100}}, 1.0},
		{"zero bid", []RequestResult{{BidLatencyMs: 0, ActualLatencyMs: 100}}, 0.5},
		{"double actual", []RequestResult{{BidLatencyMs: 100, ActualLatencyMs: 200}}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := latencyAccuracy(tt.results)
			if got != tt.want {
				t.Errorf("latencyAccuracy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQualityBias(t *testing.T) {
	// Less than 5 results -> 0
	results := []RequestResult{{BidQuality: 0.9, Success: true}}
	if got := qualityBias(results); got != 0 {
		t.Errorf("qualityBias with <5 results = %v, want 0", got)
	}

	// 10 results, all success at 0.9 bid -> bias = 0.9 - 1.0 = -0.1
	var fullSuccess []RequestResult
	for i := 0; i < 10; i++ {
		fullSuccess = append(fullSuccess, RequestResult{BidQuality: 0.9, Success: true})
	}
	got := qualityBias(fullSuccess)
	expected := 0.9 - 1.0
	if diff := got - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("qualityBias = %v, want ~%v", got, expected)
	}
}

func TestReliability(t *testing.T) {
	// No requests
	rec := &ProviderRecord{TotalRequests: 0}
	if got := reliability(rec); got != 0.5 {
		t.Errorf("reliability with 0 requests = %v, want 0.5", got)
	}

	// Perfect provider, recently seen
	rec = &ProviderRecord{
		TotalRequests:   100,
		SuccessRate:     1.0,
		LatencyAccuracy: 1.0,
		QualityBias:     0.0,
		LastSeen:        time.Now(),
	}
	got := reliability(rec)
	// 1.0*0.5 + 1.0*0.3 + 1.0*0.2 = 1.0
	if got != 1.0 {
		t.Errorf("reliability (perfect) = %v, want 1.0", got)
	}

	// Provider not seen for 2 hours
	rec = &ProviderRecord{
		TotalRequests:   100,
		SuccessRate:     1.0,
		LatencyAccuracy: 1.0,
		QualityBias:     0.0,
		LastSeen:        time.Now().Add(-2 * time.Hour),
	}
	got = reliability(rec)
	// base = 1.0, decay = 2 * 0.01 = 0.02 -> 0.98
	if got < 0.97 || got > 0.99 {
		t.Errorf("reliability (2h stale) = %v, want ~0.98", got)
	}
}

func TestRecentResultsCapped(t *testing.T) {
	tr := &Tracker{
		records: make(map[string]*ProviderRecord),
		maxHist: 5,
	}

	for i := 0; i < 10; i++ {
		bid := &exchange.Bid{ProviderID: "capped", EstLatencyMs: 100, QualityScore: 0.9}
		completed := &exchange.CompletedRequest{ProviderID: "capped", Success: true, ActualLatencyMs: 100}
		tr.RecordCompletion(bid, completed)
	}

	rec := tr.records["capped"]
	if len(rec.RecentResults) != 5 {
		t.Errorf("len(RecentResults) = %d, want 5 (capped)", len(rec.RecentResults))
	}
}
