// Package quality tracks whether providers deliver on their bid promises.
// Providers that consistently overbid on quality get penalized in future auctions.
// Providers that deliver above expectations get boosted.
package quality

import (
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
)

// Store defines the persistence interface used by the quality tracker.
type Store interface {
	SaveCompletedRequest(req exchange.CompletedRequest) error
	GetProviderQuality(providerID string) (*ProviderRecord, error)
	UpdateProviderQuality(providerID string, record *ProviderRecord) error
	ListProviderQuality() (map[string]*ProviderRecord, error)
}

// Tracker monitors provider quality over time.
type Tracker struct {
	mu       sync.RWMutex
	records  map[string]*ProviderRecord // provider ID -> record
	maxHist  int
	store    Store // optional persistence
}

// ProviderRecord holds quality history for a single provider.
type ProviderRecord struct {
	ProviderID     string        `json:"provider_id"`
	TotalRequests  int           `json:"total_requests"`
	Successes      int           `json:"successes"`
	Failures       int           `json:"failures"`
	SuccessRate    float64       `json:"success_rate"`
	AvgLatencyMs   float64       `json:"avg_latency_ms"`
	LatencyAccuracy float64      `json:"latency_accuracy"` // how close estimates are to actuals (1.0 = perfect)
	QualityBias    float64       `json:"quality_bias"`     // positive = overbids quality, negative = underbids
	ReliabilityScore float64     `json:"reliability_score"` // 0-1, used to adjust future bids
	TotalRevenue   float64       `json:"total_revenue_cents"`
	LastSeen       time.Time     `json:"last_seen"`
	RecentResults  []RequestResult `json:"-"` // internal tracking
}

// RequestResult tracks a single completed request for quality analysis.
type RequestResult struct {
	BidLatencyMs    int
	ActualLatencyMs int
	BidQuality      float64
	Success         bool
	Timestamp       time.Time
}

// New creates a quality tracker. If a Store is provided, quality records are
// loaded from it at startup and persisted on every completion.
func New(s ...Store) *Tracker {
	t := &Tracker{
		records: make(map[string]*ProviderRecord),
		maxHist: 200,
	}
	if len(s) > 0 && s[0] != nil {
		t.store = s[0]
		t.loadRecords()
	}
	return t
}

// loadRecords populates in-memory records from the store.
func (t *Tracker) loadRecords() {
	records, err := t.store.ListProviderQuality()
	if err != nil {
		log.Printf("quality: failed to load provider records: %v", err)
		return
	}
	t.records = records
	log.Printf("quality: loaded %d provider records from store", len(records))
}

// RecordCompletion records the outcome of an auction execution.
func (t *Tracker) RecordCompletion(bid *exchange.Bid, completed *exchange.CompletedRequest) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.records[completed.ProviderID]
	if !ok {
		rec = &ProviderRecord{
			ProviderID:       completed.ProviderID,
			ReliabilityScore: 0.8, // start with reasonable trust
		}
		t.records[completed.ProviderID] = rec
	}

	rec.TotalRequests++
	rec.LastSeen = time.Now()

	if completed.Success {
		rec.Successes++
	} else {
		rec.Failures++
	}
	rec.SuccessRate = float64(rec.Successes) / float64(rec.TotalRequests)
	rec.TotalRevenue += completed.ActualCost

	// Track result
	result := RequestResult{
		BidLatencyMs:    bid.EstLatencyMs,
		ActualLatencyMs: completed.ActualLatencyMs,
		BidQuality:      bid.QualityScore,
		Success:         completed.Success,
		Timestamp:       time.Now(),
	}
	rec.RecentResults = append(rec.RecentResults, result)
	if len(rec.RecentResults) > t.maxHist {
		rec.RecentResults = rec.RecentResults[len(rec.RecentResults)-t.maxHist:]
	}

	// Recalculate metrics
	rec.AvgLatencyMs = avgLatency(rec.RecentResults)
	rec.LatencyAccuracy = latencyAccuracy(rec.RecentResults)
	rec.QualityBias = qualityBias(rec.RecentResults)
	rec.ReliabilityScore = reliability(rec)

	// Persist to store
	if t.store != nil {
		if err := t.store.SaveCompletedRequest(*completed); err != nil {
			log.Printf("quality: failed to persist completed request: %v", err)
		}
		if err := t.store.UpdateProviderQuality(completed.ProviderID, rec); err != nil {
			log.Printf("quality: failed to persist provider quality for %s: %v", completed.ProviderID, err)
		}
	}
}

// GetRecord returns the quality record for a provider.
func (t *Tracker) GetRecord(providerID string) *ProviderRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if rec, ok := t.records[providerID]; ok {
		cp := *rec
		return &cp
	}
	return nil
}

// AllRecords returns quality records for all tracked providers.
func (t *Tracker) AllRecords() []ProviderRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []ProviderRecord
	for _, rec := range t.records {
		cp := *rec
		cp.RecentResults = nil // don't expose internal data
		out = append(out, cp)
	}
	return out
}

// AdjustBid modifies a provider's bid based on their quality history.
// Called before bid selection to penalize unreliable providers.
func (t *Tracker) AdjustBid(bid *exchange.Bid) {
	t.mu.RLock()
	rec, ok := t.records[bid.ProviderID]
	t.mu.RUnlock()
	if !ok {
		return // no history, use bid as-is
	}

	// Adjust quality score down if provider tends to overbid
	if rec.QualityBias > 0.05 {
		bid.QualityScore -= rec.QualityBias * 0.5 // halve the bias as penalty
		if bid.QualityScore < 0 {
			bid.QualityScore = 0
		}
	}

	// Adjust latency estimate up if provider consistently underestimates
	if rec.LatencyAccuracy < 0.8 {
		// Provider's latency estimates are inaccurate — inflate by the inaccuracy factor
		inflationFactor := 1.0 + (1.0 - rec.LatencyAccuracy)
		bid.EstLatencyMs = int(float64(bid.EstLatencyMs) * inflationFactor)
	}

	// Adjust confidence based on reliability
	bid.ConfidenceScore *= rec.ReliabilityScore
}

// =========================================================================
// Metric calculators
// =========================================================================

func avgLatency(results []RequestResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var total float64
	for _, r := range results {
		total += float64(r.ActualLatencyMs)
	}
	return total / float64(len(results))
}

// latencyAccuracy measures how close latency estimates are to actuals.
// 1.0 = perfect prediction, 0.0 = completely wrong.
func latencyAccuracy(results []RequestResult) float64 {
	if len(results) == 0 {
		return 0.5
	}
	var totalError float64
	validCount := 0
	for _, r := range results {
		if r.BidLatencyMs <= 0 {
			continue
		}
		ratio := float64(r.ActualLatencyMs) / float64(r.BidLatencyMs)
		// Error is how far from 1.0 the ratio is
		err := ratio - 1.0
		if err < 0 {
			err = -err
		}
		totalError += err
		validCount++
	}
	if validCount == 0 {
		return 0.5
	}
	avgError := totalError / float64(validCount)
	accuracy := 1.0 - avgError
	if accuracy < 0 {
		accuracy = 0
	}
	return accuracy
}

// qualityBias measures whether a provider consistently overbids on quality.
// Positive = tends to promise more than it delivers.
// Measured by comparing success rate to average bid quality score.
func qualityBias(results []RequestResult) float64 {
	if len(results) < 5 {
		return 0 // not enough data
	}
	var totalBidQuality float64
	var successCount int
	for _, r := range results {
		totalBidQuality += r.BidQuality
		if r.Success {
			successCount++
		}
	}
	avgBidQuality := totalBidQuality / float64(len(results))
	actualSuccessRate := float64(successCount) / float64(len(results))

	// Bias = how much higher they bid vs what they deliver
	return avgBidQuality - actualSuccessRate
}

// reliability computes an overall reliability score from 0-1.
func reliability(rec *ProviderRecord) float64 {
	if rec.TotalRequests == 0 {
		return 0.5
	}

	// Weight: 50% success rate, 30% latency accuracy, 20% low quality bias
	successScore := rec.SuccessRate
	latencyScore := rec.LatencyAccuracy
	biasScore := 1.0 - rec.QualityBias
	if biasScore < 0 {
		biasScore = 0
	}
	if biasScore > 1 {
		biasScore = 1
	}

	score := successScore*0.50 + latencyScore*0.30 + biasScore*0.20

	// Decay if we haven't seen them recently (more than 1 hour)
	if time.Since(rec.LastSeen) > time.Hour {
		decay := float64(time.Since(rec.LastSeen).Hours()) * 0.01
		if decay > 0.3 {
			decay = 0.3
		}
		score -= decay
	}

	if score < 0 {
		score = 0
	}
	return score
}
