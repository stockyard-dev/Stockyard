package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
	"github.com/stockyard-dev/stockyard/internal/bid/quality"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)
	// Verify tables exist by running queries against them.
	var cnt int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM auctions").Scan(&cnt); err != nil {
		t.Fatalf("auctions table missing: %v", err)
	}
	if err := db.db.QueryRow("SELECT COUNT(*) FROM bids").Scan(&cnt); err != nil {
		t.Fatalf("bids table missing: %v", err)
	}
	if err := db.db.QueryRow("SELECT COUNT(*) FROM completed_requests").Scan(&cnt); err != nil {
		t.Fatalf("completed_requests table missing: %v", err)
	}
	if err := db.db.QueryRow("SELECT COUNT(*) FROM provider_quality").Scan(&cnt); err != nil {
		t.Fatalf("provider_quality table missing: %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := openTestDB(t)
	// Running migrate again should not error.
	if err := db.migrate(); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}

func TestSaveAndListAuctions(t *testing.T) {
	db := openTestDB(t)

	result := exchange.AuctionResult{
		RequestID:   "req-1",
		BidCount:    2,
		AuctionTime: 42 * time.Millisecond,
		Reason:      "cheapest",
		Winner: &exchange.Bid{
			ProviderID: "provider-a",
			Model:      "gpt-4",
		},
		AllBids: []exchange.Bid{
			{ProviderID: "provider-a", Model: "gpt-4", PricePer1K: 0.03, EstLatencyMs: 100, QualityScore: 0.9, ConfidenceScore: 0.8},
			{ProviderID: "provider-b", Model: "claude-3", PricePer1K: 0.05, EstLatencyMs: 80, QualityScore: 0.95, ConfidenceScore: 0.85},
		},
	}

	if err := db.SaveAuction(result); err != nil {
		t.Fatalf("SaveAuction: %v", err)
	}

	auctions, err := db.ListAuctions(10)
	if err != nil {
		t.Fatalf("ListAuctions: %v", err)
	}
	if len(auctions) != 1 {
		t.Fatalf("expected 1 auction, got %d", len(auctions))
	}

	a := auctions[0]
	if a.RequestID != "req-1" {
		t.Errorf("RequestID = %q, want %q", a.RequestID, "req-1")
	}
	if a.BidCount != 2 {
		t.Errorf("BidCount = %d, want 2", a.BidCount)
	}
	if a.AuctionTime != 42*time.Millisecond {
		t.Errorf("AuctionTime = %v, want 42ms", a.AuctionTime)
	}
	if a.Reason != "cheapest" {
		t.Errorf("Reason = %q, want %q", a.Reason, "cheapest")
	}
	if a.Winner == nil {
		t.Fatal("Winner is nil")
	}
	if a.Winner.ProviderID != "provider-a" {
		t.Errorf("Winner.ProviderID = %q, want %q", a.Winner.ProviderID, "provider-a")
	}
	if len(a.AllBids) != 2 {
		t.Fatalf("expected 2 bids, got %d", len(a.AllBids))
	}
	if a.AllBids[0].PricePer1K != 0.03 {
		t.Errorf("bid[0].PricePer1K = %f, want 0.03", a.AllBids[0].PricePer1K)
	}
}

func TestSaveAuctionNilWinner(t *testing.T) {
	db := openTestDB(t)

	result := exchange.AuctionResult{
		RequestID:    "req-no-winner",
		BidCount:     0,
		AuctionTime:  10 * time.Millisecond,
		NoBidsReason: "no providers submitted bids",
	}

	if err := db.SaveAuction(result); err != nil {
		t.Fatalf("SaveAuction: %v", err)
	}

	auctions, err := db.ListAuctions(10)
	if err != nil {
		t.Fatalf("ListAuctions: %v", err)
	}
	if len(auctions) != 1 {
		t.Fatalf("expected 1 auction, got %d", len(auctions))
	}
	if auctions[0].Winner != nil {
		t.Error("expected nil winner for auction with no bids")
	}
	if auctions[0].Reason != "no providers submitted bids" {
		t.Errorf("Reason = %q, want NoBidsReason fallback", auctions[0].Reason)
	}
}

func TestListAuctionsEmpty(t *testing.T) {
	db := openTestDB(t)
	auctions, err := db.ListAuctions(10)
	if err != nil {
		t.Fatalf("ListAuctions: %v", err)
	}
	if len(auctions) != 0 {
		t.Fatalf("expected 0 auctions, got %d", len(auctions))
	}
}

func TestListAuctionsLimit(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 5; i++ {
		result := exchange.AuctionResult{
			RequestID:   "req-" + string(rune('a'+i)),
			BidCount:    1,
			AuctionTime: time.Millisecond,
			Reason:      "test",
		}
		if err := db.SaveAuction(result); err != nil {
			t.Fatalf("SaveAuction %d: %v", i, err)
		}
	}

	auctions, err := db.ListAuctions(3)
	if err != nil {
		t.Fatalf("ListAuctions: %v", err)
	}
	if len(auctions) != 3 {
		t.Fatalf("expected 3 auctions, got %d", len(auctions))
	}
}

func TestDuplicateAuctionID(t *testing.T) {
	db := openTestDB(t)

	result := exchange.AuctionResult{
		RequestID:   "dup-id",
		BidCount:    1,
		AuctionTime: time.Millisecond,
		Reason:      "test",
	}
	if err := db.SaveAuction(result); err != nil {
		t.Fatalf("first SaveAuction: %v", err)
	}
	// Inserting the same ID should fail (PRIMARY KEY constraint).
	err := db.SaveAuction(result)
	if err == nil {
		t.Fatal("expected error on duplicate auction ID, got nil")
	}
}

func TestSaveCompletedRequest(t *testing.T) {
	db := openTestDB(t)

	req := exchange.CompletedRequest{
		RequestID:       "req-1",
		ProviderID:      "provider-a",
		Model:           "gpt-4",
		BidPrice:        0.03,
		ActualLatencyMs: 120,
		ActualTokens:    500,
		ActualCost:      0.015,
		Success:         true,
	}

	if err := db.SaveCompletedRequest(req); err != nil {
		t.Fatalf("SaveCompletedRequest: %v", err)
	}

	// Verify the row exists.
	var count int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM completed_requests WHERE auction_id = ?", "req-1").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 completed_request row, got %d", count)
	}
}

func TestSaveCompletedRequestWithError(t *testing.T) {
	db := openTestDB(t)

	req := exchange.CompletedRequest{
		RequestID:  "req-err",
		ProviderID: "provider-x",
		Model:      "bad-model",
		Success:    false,
		Error:      "connection timeout",
	}

	if err := db.SaveCompletedRequest(req); err != nil {
		t.Fatalf("SaveCompletedRequest: %v", err)
	}

	var errStr string
	if err := db.db.QueryRow("SELECT error FROM completed_requests WHERE auction_id = ?", "req-err").Scan(&errStr); err != nil {
		t.Fatalf("query: %v", err)
	}
	if errStr != "connection timeout" {
		t.Errorf("error = %q, want %q", errStr, "connection timeout")
	}
}

func TestProviderQualityCRUD(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	rec := &quality.ProviderRecord{
		ProviderID:    "prov-1",
		TotalRequests: 100,
		Successes:     90,
		Failures:      10,
		AvgLatencyMs:  150.5,
		QualityBias:   0.05,
		TotalRevenue:  42.0,
		LastSeen:      now,
	}

	// Insert
	if err := db.UpdateProviderQuality("prov-1", rec); err != nil {
		t.Fatalf("UpdateProviderQuality (insert): %v", err)
	}

	// Read
	got, err := db.GetProviderQuality("prov-1")
	if err != nil {
		t.Fatalf("GetProviderQuality: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil record")
	}
	if got.TotalRequests != 100 {
		t.Errorf("TotalRequests = %d, want 100", got.TotalRequests)
	}
	if got.Successes != 90 {
		t.Errorf("Successes = %d, want 90", got.Successes)
	}
	if got.SuccessRate != 0.9 {
		t.Errorf("SuccessRate = %f, want 0.9", got.SuccessRate)
	}
	if got.AvgLatencyMs != 150.5 {
		t.Errorf("AvgLatencyMs = %f, want 150.5", got.AvgLatencyMs)
	}

	// Update (upsert)
	rec.TotalRequests = 200
	rec.Successes = 180
	if err := db.UpdateProviderQuality("prov-1", rec); err != nil {
		t.Fatalf("UpdateProviderQuality (upsert): %v", err)
	}
	got, err = db.GetProviderQuality("prov-1")
	if err != nil {
		t.Fatalf("GetProviderQuality after update: %v", err)
	}
	if got.TotalRequests != 200 {
		t.Errorf("TotalRequests after update = %d, want 200", got.TotalRequests)
	}
}

func TestGetProviderQualityNotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetProviderQuality("nonexistent")
	if err != nil {
		t.Fatalf("GetProviderQuality: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent provider, got %+v", got)
	}
}

func TestListProviderQuality(t *testing.T) {
	db := openTestDB(t)

	providers := []struct {
		id  string
		rec *quality.ProviderRecord
	}{
		{"p1", &quality.ProviderRecord{TotalRequests: 10, Successes: 9, Failures: 1, AvgLatencyMs: 100}},
		{"p2", &quality.ProviderRecord{TotalRequests: 20, Successes: 15, Failures: 5, AvgLatencyMs: 200}},
		{"p3", &quality.ProviderRecord{TotalRequests: 5, Successes: 5, Failures: 0, AvgLatencyMs: 50}},
	}

	for _, p := range providers {
		if err := db.UpdateProviderQuality(p.id, p.rec); err != nil {
			t.Fatalf("UpdateProviderQuality(%s): %v", p.id, err)
		}
	}

	records, err := db.ListProviderQuality()
	if err != nil {
		t.Fatalf("ListProviderQuality: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records["p1"].TotalRequests != 10 {
		t.Errorf("p1 TotalRequests = %d, want 10", records["p1"].TotalRequests)
	}
	if records["p2"].SuccessRate != 0.75 {
		t.Errorf("p2 SuccessRate = %f, want 0.75", records["p2"].SuccessRate)
	}
	if records["p3"].SuccessRate != 1.0 {
		t.Errorf("p3 SuccessRate = %f, want 1.0", records["p3"].SuccessRate)
	}
}

func TestListProviderQualityEmpty(t *testing.T) {
	db := openTestDB(t)
	records, err := db.ListProviderQuality()
	if err != nil {
		t.Fatalf("ListProviderQuality: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
