package tracker

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

func TestSpendCounter(t *testing.T) {
	sc := NewSpendCounter()

	// Start at zero
	spend := sc.Get("project-a")
	if spend.Today != 0 || spend.Month != 0 {
		t.Errorf("initial spend = today:%.4f month:%.4f, want 0,0", spend.Today, spend.Month)
	}

	// Add spend
	sc.Add("project-a", 1.50)
	sc.Add("project-a", 0.75)
	spend = sc.Get("project-a")
	if spend.Today != 2.25 {
		t.Errorf("today = %.4f, want 2.25", spend.Today)
	}
	if spend.Month != 2.25 {
		t.Errorf("month = %.4f, want 2.25", spend.Month)
	}

	// Different project is isolated
	sc.Add("project-b", 10.00)
	spendA := sc.Get("project-a")
	spendB := sc.Get("project-b")
	if spendA.Today != 2.25 {
		t.Errorf("project-a today = %.4f, want 2.25", spendA.Today)
	}
	if spendB.Today != 10.00 {
		t.Errorf("project-b today = %.4f, want 10.00", spendB.Today)
	}

	// GetAll returns all projects
	all := sc.GetAll()
	if len(all) != 2 {
		t.Errorf("GetAll len = %d, want 2", len(all))
	}
}

func TestCalculateRequestCost(t *testing.T) {
	// gpt-4o-mini: $0.15/1M in, $0.60/1M out
	cost := CalculateRequestCost("gpt-4o-mini", 1000, 500)
	// 1000 * 0.15/1M + 500 * 0.60/1M = 0.000150 + 0.000300 = 0.000450
	if cost < 0.00044 || cost > 0.00046 {
		t.Errorf("cost = %f, want ~0.000450", cost)
	}
}

func TestEstimateRequestCost(t *testing.T) {
	msgs := []provider.Message{{Role: "user", Content: "Hello, how are you?"}}
	cost := EstimateRequestCost("gpt-4o-mini", msgs)
	if cost <= 0 {
		t.Errorf("estimated cost = %f, want > 0", cost)
	}
}
