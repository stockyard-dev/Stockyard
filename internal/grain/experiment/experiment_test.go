package experiment

import (
	"math"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/grain/audit"
	"github.com/stockyard-dev/stockyard/internal/grain/store"
	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

func makeEntries(decisionID string, variants map[string]int) []audit.Entry {
	var entries []audit.Entry
	for v, count := range variants {
		for i := 0; i < count; i++ {
			entries = append(entries, audit.Entry{
				Outcome: tree.Outcome{
					DecisionID: decisionID,
					Variant:    v,
				},
			})
		}
	}
	return entries
}

func makeOutcomes(decisionID string, conversions map[string]int, errors map[string]int) []store.Outcome {
	var outcomes []store.Outcome
	for v, count := range conversions {
		for i := 0; i < count; i++ {
			outcomes = append(outcomes, store.Outcome{
				DecisionID: decisionID,
				Variant:    v,
				Outcome:    "conversion",
			})
		}
	}
	for v, count := range errors {
		for i := 0; i < count; i++ {
			outcomes = append(outcomes, store.Outcome{
				DecisionID: decisionID,
				Variant:    v,
				Outcome:    "error",
			})
		}
	}
	return outcomes
}

func TestAnalyzeExperiment_Basic(t *testing.T) {
	entries := makeEntries("d1", map[string]int{"A": 100, "B": 100})
	outcomes := makeOutcomes("d1", map[string]int{"A": 60, "B": 40}, nil)

	result := AnalyzeExperiment("d1", entries, outcomes)

	if result.DecisionID != "d1" {
		t.Errorf("expected decision d1, got %s", result.DecisionID)
	}
	if len(result.VariantStats) != 2 {
		t.Errorf("expected 2 variants, got %d", len(result.VariantStats))
	}
	if result.Winner != "A" {
		t.Errorf("expected winner A, got %s", result.Winner)
	}

	statA := result.VariantStats["A"]
	if statA.SampleSize != 100 {
		t.Errorf("expected sample size 100 for A, got %d", statA.SampleSize)
	}
	if math.Abs(statA.ConversionRate-0.6) > 0.001 {
		t.Errorf("expected conversion rate 0.6 for A, got %f", statA.ConversionRate)
	}
}

func TestAnalyzeExperiment_EmptyEntries(t *testing.T) {
	result := AnalyzeExperiment("d1", nil, nil)

	if result.DecisionID != "d1" {
		t.Errorf("expected decision d1, got %s", result.DecisionID)
	}
	if len(result.VariantStats) != 0 {
		t.Errorf("expected 0 variants, got %d", len(result.VariantStats))
	}
	if result.Winner != "" {
		t.Errorf("expected empty winner, got %s", result.Winner)
	}
}

func TestAnalyzeExperiment_FiltersDecision(t *testing.T) {
	entries := []audit.Entry{
		{Outcome: tree.Outcome{DecisionID: "d1", Variant: "A"}},
		{Outcome: tree.Outcome{DecisionID: "other", Variant: "A"}},
	}
	result := AnalyzeExperiment("d1", entries, nil)

	if result.VariantStats["A"].SampleSize != 1 {
		t.Errorf("expected 1 entry for A (filtered), got %d", result.VariantStats["A"].SampleSize)
	}
}

func TestAnalyzeExperiment_DefaultVariant(t *testing.T) {
	entries := []audit.Entry{
		{Outcome: tree.Outcome{DecisionID: "d1", Variant: ""}},
	}
	result := AnalyzeExperiment("d1", entries, nil)

	if _, ok := result.VariantStats["__default__"]; !ok {
		t.Error("expected __default__ variant for empty variant name")
	}
}

func TestAnalyzeExperiment_ErrorRate(t *testing.T) {
	entries := makeEntries("d1", map[string]int{"A": 100})
	outcomes := makeOutcomes("d1", nil, map[string]int{"A": 10})

	result := AnalyzeExperiment("d1", entries, outcomes)

	statA := result.VariantStats["A"]
	if math.Abs(statA.ErrorRate-0.1) > 0.001 {
		t.Errorf("expected error rate 0.1, got %f", statA.ErrorRate)
	}
}

func TestAnalyzeExperiment_ResponseTime(t *testing.T) {
	entries := makeEntries("d1", map[string]int{"A": 2})
	outcomes := []store.Outcome{
		{DecisionID: "d1", Variant: "A", Outcome: "conversion", Value: 100},
		{DecisionID: "d1", Variant: "A", Outcome: "conversion", Value: 200},
	}

	result := AnalyzeExperiment("d1", entries, outcomes)

	statA := result.VariantStats["A"]
	if math.Abs(statA.AvgResponseTime-150) > 0.001 {
		t.Errorf("expected avg response time 150, got %f", statA.AvgResponseTime)
	}
}

func TestAnalyzeExperiment_Significance(t *testing.T) {
	// Large sample with clear difference — verify significance is computed without panic
	entries := makeEntries("d1", map[string]int{"A": 1000, "B": 1000})
	outcomes := makeOutcomes("d1", map[string]int{"A": 600, "B": 400}, nil)

	result := AnalyzeExperiment("d1", entries, outcomes)

	// The function should compute without error; verify fields are populated
	if result.Winner != "A" {
		t.Errorf("expected winner A, got %s", result.Winner)
	}
	// Confidence should be in [0, 1]
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("expected confidence in [0,1], got %f", result.Confidence)
	}
}

func TestIsSignificant_SmallSample(t *testing.T) {
	a := VariantStat{SampleSize: 5, ConversionRate: 0.8}
	b := VariantStat{SampleSize: 5, ConversionRate: 0.2}

	sig, _ := IsSignificant(a, b)
	// Small sample: may or may not be significant, but shouldn't panic
	_ = sig
}

func TestIsSignificant_ZeroSample(t *testing.T) {
	a := VariantStat{SampleSize: 0, ConversionRate: 0}
	b := VariantStat{SampleSize: 100, ConversionRate: 0.5}

	sig, pval := IsSignificant(a, b)
	if sig {
		t.Error("expected not significant with zero sample")
	}
	if pval != 1.0 {
		t.Errorf("expected p-value 1.0, got %f", pval)
	}
}

func TestIsSignificant_EqualRates(t *testing.T) {
	a := VariantStat{SampleSize: 1000, ConversionRate: 0.5}
	b := VariantStat{SampleSize: 1000, ConversionRate: 0.5}

	sig, _ := IsSignificant(a, b)
	if sig {
		t.Error("equal rates should not be significant")
	}
}

func TestIsSignificant_ZeroConversions(t *testing.T) {
	a := VariantStat{SampleSize: 100, ConversionRate: 0}
	b := VariantStat{SampleSize: 100, ConversionRate: 0}

	sig, pval := IsSignificant(a, b)
	if sig {
		t.Error("zero conversions should not be significant")
	}
	if pval != 1.0 {
		t.Errorf("expected p-value 1.0 for zero pool, got %f", pval)
	}
}

func TestIsSignificant_AllConversions(t *testing.T) {
	a := VariantStat{SampleSize: 100, ConversionRate: 1.0}
	b := VariantStat{SampleSize: 100, ConversionRate: 1.0}

	sig, pval := IsSignificant(a, b)
	if sig {
		t.Error("all conversions should not be significant when equal")
	}
	if pval != 1.0 {
		t.Errorf("expected p-value 1.0 for pool=1, got %f", pval)
	}
}

func TestNormalCDF(t *testing.T) {
	// Verify normalCDF returns values in [0,1] and increases for large x.
	for _, x := range []float64{-5, -3, -1, 0, 1, 3, 5} {
		val := normalCDF(x)
		if val < 0 || val > 1 {
			t.Errorf("normalCDF(%g) = %g, out of [0,1]", x, val)
		}
	}
	// For large positive x, should approach 1
	if normalCDF(5) < 0.9 {
		t.Errorf("normalCDF(5) should be close to 1, got %g", normalCDF(5))
	}
}
