package learn

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
)

func TestNew(t *testing.T) {
	c := New(nil)
	if c == nil {
		t.Fatal("New(nil) returned nil")
	}
	if c.maxPairs != 2000 {
		t.Errorf("maxPairs = %d, want 2000", c.maxPairs)
	}
	if c.dimensionBias == nil {
		t.Error("dimensionBias is nil")
	}
}

func TestAdjustScore(t *testing.T) {
	c := New(nil)

	// No bias - score should stay the same
	got := c.AdjustScore(0.5)
	if got != 0.5 {
		t.Errorf("AdjustScore(0.5) with no bias = %f, want 0.5", got)
	}

	// Set positive bias
	c.mu.Lock()
	c.biasShift = 0.1
	c.mu.Unlock()

	got = c.AdjustScore(0.5)
	if got != 0.6 {
		t.Errorf("AdjustScore(0.5) with +0.1 bias = %f, want 0.6", got)
	}

	// Clamp to 1.0
	got = c.AdjustScore(0.95)
	if got != 1.0 {
		t.Errorf("AdjustScore(0.95) with +0.1 bias = %f, want 1.0 (clamped)", got)
	}

	// Negative bias clamped to 0
	c.mu.Lock()
	c.biasShift = -0.8
	c.mu.Unlock()

	got = c.AdjustScore(0.3)
	if got != 0.0 {
		t.Errorf("AdjustScore(0.3) with -0.8 bias = %f, want 0.0 (clamped)", got)
	}
}

func TestAdjustDimension(t *testing.T) {
	c := New(nil)

	// No bias
	got := c.AdjustDimension("relevance", 0.7)
	if got != 0.7 {
		t.Errorf("AdjustDimension with no bias = %f, want 0.7", got)
	}

	// Set dimension bias
	c.mu.Lock()
	c.dimensionBias["relevance"] = 0.1
	c.mu.Unlock()

	got = c.AdjustDimension("relevance", 0.7)
	if got < 0.799 || got > 0.801 {
		t.Errorf("AdjustDimension with +0.1 bias = %f, want ~0.8", got)
	}

	// Clamp to 1.0
	got = c.AdjustDimension("relevance", 0.95)
	if got != 1.0 {
		t.Errorf("AdjustDimension clamped = %f, want 1.0", got)
	}

	// Clamp to 0.0
	c.mu.Lock()
	c.dimensionBias["accuracy"] = -1.0
	c.mu.Unlock()
	got = c.AdjustDimension("accuracy", 0.3)
	if got != 0.0 {
		t.Errorf("AdjustDimension clamped = %f, want 0.0", got)
	}
}

func TestRecordFeedback_HappyReactions(t *testing.T) {
	c := New(nil)

	happyReactions := []string{"accepted", "thumbs_up"}
	for _, reaction := range happyReactions {
		eval := &judge.Evaluation{
			Score: 0.8,
			Dimensions: judge.Scores{
				Relevance: 0.8, Accuracy: 0.9, Helpfulness: 0.7, Safety: 1.0, Tone: 0.8,
			},
		}
		c.RecordFeedback(eval, Feedback{RequestID: "req-" + reaction, Reaction: reaction})
	}

	if len(c.pairs) != 2 {
		t.Errorf("pairs len = %d, want 2", len(c.pairs))
	}
	for _, p := range c.pairs {
		if !p.Happy {
			t.Errorf("pair for reaction %q should be happy", p.Reaction)
		}
	}
}

func TestRecordFeedback_UnhappyReactions(t *testing.T) {
	c := New(nil)

	unhappyReactions := []string{"rephrased", "escalated", "abandoned", "thumbs_down"}
	for _, reaction := range unhappyReactions {
		eval := &judge.Evaluation{
			Score: 0.3,
			Dimensions: judge.Scores{
				Relevance: 0.3, Accuracy: 0.2, Helpfulness: 0.4, Safety: 1.0, Tone: 0.5,
			},
		}
		c.RecordFeedback(eval, Feedback{RequestID: "req-" + reaction, Reaction: reaction})
	}

	for _, p := range c.pairs {
		if p.Happy {
			t.Errorf("pair for reaction %q should not be happy", p.Reaction)
		}
	}
}

func TestRecordFeedback_MaxPairs(t *testing.T) {
	c := New(nil)
	c.maxPairs = 10

	for i := 0; i < 20; i++ {
		eval := &judge.Evaluation{
			Score: 0.5,
			Dimensions: judge.Scores{
				Relevance: 0.5, Accuracy: 0.5, Helpfulness: 0.5, Safety: 0.5, Tone: 0.5,
			},
		}
		reaction := "accepted"
		if i%2 == 0 {
			reaction = "thumbs_down"
		}
		c.RecordFeedback(eval, Feedback{RequestID: "req", Reaction: reaction})
	}

	if len(c.pairs) > 10 {
		t.Errorf("pairs len = %d, want <= 10 (maxPairs)", len(c.pairs))
	}
}

func TestRecalibrate(t *testing.T) {
	c := New(nil)

	// Add enough pairs to trigger recalibration
	for i := 0; i < 15; i++ {
		score := 0.8
		reaction := "accepted"
		if i < 5 {
			score = 0.3
			reaction = "thumbs_down"
		}
		eval := &judge.Evaluation{
			Score: score,
			Dimensions: judge.Scores{
				Relevance: score, Accuracy: score, Helpfulness: score, Safety: score, Tone: score,
			},
		}
		c.RecordFeedback(eval, Feedback{RequestID: "req", Reaction: reaction})
	}

	// After recalibration, biasShift should be non-zero
	c.mu.RLock()
	bias := c.biasShift
	c.mu.RUnlock()

	// The bias should exist (could be positive or negative)
	// With happy avg=0.8 and unhappy avg=0.3, midpoint=0.55, ideal=0.65
	// bias = (0.65 - 0.55) * 0.5 = 0.05
	if bias == 0 {
		t.Error("biasShift should be non-zero after recalibration")
	}
}

func TestRecalibrate_NotEnoughPairs(t *testing.T) {
	c := New(nil)

	for i := 0; i < 5; i++ {
		eval := &judge.Evaluation{
			Score: 0.5,
			Dimensions: judge.Scores{
				Relevance: 0.5, Accuracy: 0.5, Helpfulness: 0.5, Safety: 0.5, Tone: 0.5,
			},
		}
		c.RecordFeedback(eval, Feedback{RequestID: "req", Reaction: "accepted"})
	}

	c.mu.RLock()
	bias := c.biasShift
	c.mu.RUnlock()

	if bias != 0 {
		t.Errorf("biasShift = %f, want 0 (not enough pairs)", bias)
	}
}

func TestStats_Empty(t *testing.T) {
	c := New(nil)
	s := c.Stats()
	if s.TotalFeedback != 0 {
		t.Errorf("TotalFeedback = %d, want 0", s.TotalFeedback)
	}
	if s.HappyRate != 0 {
		t.Errorf("HappyRate = %f, want 0", s.HappyRate)
	}
}

func TestStats_WithData(t *testing.T) {
	c := New(nil)

	// Add happy pairs with high scores, unhappy with low scores
	for i := 0; i < 10; i++ {
		score := 0.8
		reaction := "accepted"
		if i < 3 {
			score = 0.3
			reaction = "thumbs_down"
		}
		eval := &judge.Evaluation{
			Score: score,
			Dimensions: judge.Scores{
				Relevance: score, Accuracy: score, Helpfulness: score, Safety: score, Tone: score,
			},
		}
		c.RecordFeedback(eval, Feedback{RequestID: "req", Reaction: reaction})
	}

	s := c.Stats()
	if s.TotalFeedback != 10 {
		t.Errorf("TotalFeedback = %d, want 10", s.TotalFeedback)
	}
	if s.HappyRate != 0.7 {
		t.Errorf("HappyRate = %f, want 0.7", s.HappyRate)
	}
	if s.Accuracy == 0 {
		t.Error("Accuracy should be non-zero")
	}
	if s.DimensionBias == nil {
		t.Error("DimensionBias is nil")
	}
}
