// Package experiment provides statistical analysis for A/B tested decisions.
// It computes per-variant statistics and uses a two-proportion z-test to
// determine whether one variant significantly outperforms another.
package experiment

import (
	"math"
	"time"

	"github.com/stockyard-dev/stockyard/internal/grain/audit"
	"github.com/stockyard-dev/stockyard/internal/grain/store"
)

// Experiment describes an active A/B test on a decision.
type Experiment struct {
	DecisionID string            `json:"decision_id"`
	StartedAt  time.Time         `json:"started_at"`
	Variants   []string          `json:"variants"`
	SampleSize map[string]int    `json:"sample_size"`
}

// VariantStat holds per-variant metrics.
type VariantStat struct {
	SampleSize      int     `json:"sample_size"`
	ConversionRate  float64 `json:"conversion_rate"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
	ErrorRate       float64 `json:"error_rate"`
}

// ExperimentResult is the output of AnalyzeExperiment.
type ExperimentResult struct {
	DecisionID         string                  `json:"decision_id"`
	VariantStats       map[string]VariantStat  `json:"variant_stats"`
	Winner             string                  `json:"winner"`
	Confidence         float64                 `json:"confidence"`
	SignificanceReached bool                   `json:"significance_reached"`
}

// AnalyzeExperiment computes variant-level statistics and determines a winner.
// It uses audit entries for traffic counts and store outcomes for conversions.
func AnalyzeExperiment(decisionID string, entries []audit.Entry, outcomes []store.Outcome) *ExperimentResult {
	result := &ExperimentResult{
		DecisionID:   decisionID,
		VariantStats: make(map[string]VariantStat),
	}

	// Count evaluations per variant from audit entries.
	variantCounts := map[string]int{}
	variantResponseTimes := map[string][]float64{}
	variantErrors := map[string]int{}

	for _, e := range entries {
		if e.Outcome.DecisionID != decisionID {
			continue
		}
		v := e.Outcome.Variant
		if v == "" {
			v = "__default__"
		}
		variantCounts[v]++
	}

	// Count conversions per variant from outcomes.
	variantConversions := map[string]int{}
	for _, o := range outcomes {
		if o.DecisionID != decisionID {
			continue
		}
		v := o.Variant
		if v == "" {
			v = "__default__"
		}
		switch o.Outcome {
		case "conversion", "success":
			variantConversions[v]++
		case "error", "failure":
			variantErrors[v]++
		}
		if o.Value > 0 {
			variantResponseTimes[v] = append(variantResponseTimes[v], o.Value)
		}
	}

	// Build stats per variant.
	bestRate := -1.0
	bestVariant := ""
	for v, count := range variantCounts {
		stat := VariantStat{SampleSize: count}
		if count > 0 {
			stat.ConversionRate = float64(variantConversions[v]) / float64(count)
			stat.ErrorRate = float64(variantErrors[v]) / float64(count)
		}
		if times := variantResponseTimes[v]; len(times) > 0 {
			sum := 0.0
			for _, t := range times {
				sum += t
			}
			stat.AvgResponseTime = sum / float64(len(times))
		}
		result.VariantStats[v] = stat
		if stat.ConversionRate > bestRate {
			bestRate = stat.ConversionRate
			bestVariant = v
		}
	}

	result.Winner = bestVariant

	// If there are at least two variants, check significance of the best pair.
	if len(result.VariantStats) >= 2 {
		var secondBest string
		secondRate := -1.0
		for v, stat := range result.VariantStats {
			if v != bestVariant && stat.ConversionRate > secondRate {
				secondRate = stat.ConversionRate
				secondBest = v
			}
		}
		if secondBest != "" {
			sig, pval := IsSignificant(result.VariantStats[bestVariant], result.VariantStats[secondBest])
			result.SignificanceReached = sig
			result.Confidence = 1 - pval
		}
	}

	return result
}

// IsSignificant performs a two-proportion z-test and returns whether the
// difference is statistically significant at p < 0.05.
func IsSignificant(a, b VariantStat) (bool, float64) {
	n1 := float64(a.SampleSize)
	n2 := float64(b.SampleSize)
	if n1 < 1 || n2 < 1 {
		return false, 1.0
	}
	p1 := a.ConversionRate
	p2 := b.ConversionRate

	// Pooled proportion.
	pPool := (p1*n1 + p2*n2) / (n1 + n2)
	if pPool <= 0 || pPool >= 1 {
		return false, 1.0
	}

	se := math.Sqrt(pPool * (1 - pPool) * (1/n1 + 1/n2))
	if se == 0 {
		return false, 1.0
	}

	z := math.Abs(p1-p2) / se

	// Two-tailed p-value using the standard normal CDF approximation.
	pValue := 2 * (1 - normalCDF(z))
	return pValue < 0.05, pValue
}

// normalCDF approximates the cumulative distribution function of the standard
// normal distribution using the Abramowitz and Stegun rational approximation.
func normalCDF(x float64) float64 {
	if x < 0 {
		return 1 - normalCDF(-x)
	}
	// Constants for the approximation.
	const (
		a1 = 0.254829592
		a2 = -0.284496736
		a3 = 1.421413741
		a4 = -1.453152027
		a5 = 1.061405429
		p  = 0.3275911
	)
	t := 1.0 / (1.0 + p*x)
	poly := ((((a5*t+a4)*t+a3)*t+a2)*t + a1) * t
	return 1.0 - poly*math.Exp(-x*x/2.0)
}
