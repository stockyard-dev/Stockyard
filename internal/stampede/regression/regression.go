// Package regression provides statistical comparison between simulation runs.
package regression

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/stampede/store"
)

// ComparisonResult holds the full regression analysis between two runs.
type ComparisonResult struct {
	SimA    string           `json:"sim_a"`
	SimB    string           `json:"sim_b"`
	Overall MetricComparison `json:"overall"`
	ByPersona map[string]MetricComparison `json:"by_persona"`
	Safety    SafetyComparison `json:"safety"`
	NewFailurePatterns []FailurePatternDiff `json:"new_failure_patterns"`
	Verdict   string           `json:"verdict"` // regression, improvement, no_change
	VerdictDetail string       `json:"verdict_detail"`
}

// MetricComparison compares a metric between two runs with significance.
type MetricComparison struct {
	Before      float64 `json:"before"`
	After       float64 `json:"after"`
	Delta       float64 `json:"delta"`
	PValue      float64 `json:"p_value"`
	Significant bool    `json:"significant"` // p < 0.05
	Direction   string  `json:"direction"`   // improved, regressed, unchanged
}

// SafetyComparison compares safety findings between runs.
type SafetyComparison struct {
	CriticalBefore int `json:"critical_before"`
	CriticalAfter  int `json:"critical_after"`
	HighBefore     int `json:"high_before"`
	HighAfter      int `json:"high_after"`
	MediumBefore   int `json:"medium_before"`
	MediumAfter    int `json:"medium_after"`
	Direction      string `json:"direction"` // improved, regressed, unchanged
}

// FailurePatternDiff identifies failure patterns that are new or changed.
type FailurePatternDiff struct {
	Pattern     string  `json:"pattern"`
	CountBefore int     `json:"count_before"`
	CountAfter  int     `json:"count_after"`
	Delta       int     `json:"delta"`
	Status      string  `json:"status"` // new, worsened, improved, resolved
}

// Compare performs a statistical comparison between two simulation results.
func Compare(a, b *store.SimulationResult) *ComparisonResult {
	r := &ComparisonResult{
		SimA:      a.ID,
		SimB:      b.ID,
		ByPersona: map[string]MetricComparison{},
	}

	// Overall completion rate comparison
	rateA := completionRate(a)
	rateB := completionRate(b)
	nA := len(a.Conversations)
	nB := len(b.Conversations)
	succA := countCompleted(a)
	succB := countCompleted(b)

	r.Overall = MetricComparison{
		Before: rateA,
		After:  rateB,
		Delta:  rateB - rateA,
		PValue: proportionZTest(succA, nA, succB, nB),
	}
	r.Overall.Significant = r.Overall.PValue < 0.05
	r.Overall.Direction = direction(r.Overall.Delta, r.Overall.Significant)

	// Per-persona comparison
	allNames := map[string]bool{}
	for n := range a.PersonaBreakdown {
		allNames[n] = true
	}
	for n := range b.PersonaBreakdown {
		allNames[n] = true
	}

	for name := range allNames {
		pa := a.PersonaBreakdown[name]
		pb := b.PersonaBreakdown[name]

		mc := MetricComparison{
			Before: pa.Rate,
			After:  pb.Rate,
			Delta:  pb.Rate - pa.Rate,
			PValue: proportionZTest(pa.Completed, pa.Total, pb.Completed, pb.Total),
		}
		mc.Significant = mc.PValue < 0.05
		mc.Direction = direction(mc.Delta, mc.Significant)
		r.ByPersona[name] = mc
	}

	// Safety comparison
	r.Safety = compareSafety(a, b)

	// Failure pattern diffing
	r.NewFailurePatterns = diffFailurePatterns(a, b)

	// Overall verdict
	r.Verdict, r.VerdictDetail = computeVerdict(r)

	return r
}

// Terminal prints a formatted regression report to stdout.
func Terminal(r *ComparisonResult) {
	fmt.Println()
	fmt.Println("  ═══ REGRESSION REPORT ═════════════════════════════════════════")
	fmt.Printf("  Comparing: %s (before) vs %s (after)\n", r.SimA, r.SimB)
	fmt.Println()

	// Overall
	fmt.Println("  GOAL COMPLETION (overall)")
	printMetric("    Overall", r.Overall)

	// Per persona
	fmt.Println()
	fmt.Println("  GOAL COMPLETION (by persona)")
	var names []string
	for n := range r.ByPersona {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		printMetric("    "+name, r.ByPersona[name])
	}

	// Safety
	if r.Safety.CriticalBefore > 0 || r.Safety.CriticalAfter > 0 ||
		r.Safety.HighBefore > 0 || r.Safety.HighAfter > 0 {
		fmt.Println()
		fmt.Println("  SAFETY")
		if r.Safety.CriticalBefore > 0 || r.Safety.CriticalAfter > 0 {
			fmt.Printf("    Critical: %d → %d", r.Safety.CriticalBefore, r.Safety.CriticalAfter)
			printSafetyDirection(r.Safety.CriticalAfter - r.Safety.CriticalBefore)
			fmt.Println()
		}
		if r.Safety.HighBefore > 0 || r.Safety.HighAfter > 0 {
			fmt.Printf("    High:     %d → %d", r.Safety.HighBefore, r.Safety.HighAfter)
			printSafetyDirection(r.Safety.HighAfter - r.Safety.HighBefore)
			fmt.Println()
		}
		if r.Safety.MediumBefore > 0 || r.Safety.MediumAfter > 0 {
			fmt.Printf("    Medium:   %d → %d\n", r.Safety.MediumBefore, r.Safety.MediumAfter)
		}
	}

	// New/worsened failure patterns
	newPatterns := filterPatterns(r.NewFailurePatterns, "new", "worsened")
	if len(newPatterns) > 0 {
		fmt.Println()
		fmt.Println("  NEW/WORSENED FAILURE PATTERNS")
		for _, p := range newPatterns {
			if p.Status == "new" {
				fmt.Printf("    │ NEW: %s (%d occurrences)\n", p.Pattern, p.CountAfter)
			} else {
				fmt.Printf("    │ WORSENED: %s (%d → %d, +%d)\n", p.Pattern, p.CountBefore, p.CountAfter, p.Delta)
			}
		}
	}

	// Resolved patterns
	resolved := filterPatterns(r.NewFailurePatterns, "resolved")
	if len(resolved) > 0 {
		fmt.Println()
		fmt.Println("  RESOLVED FAILURE PATTERNS")
		for _, p := range resolved {
			fmt.Printf("    │ ✓ %s (was %d, now 0)\n", p.Pattern, p.CountBefore)
		}
	}

	// Verdict
	fmt.Println()
	switch r.Verdict {
	case "regression":
		fmt.Printf("  VERDICT: ⚠ REGRESSION DETECTED\n")
	case "improvement":
		fmt.Printf("  VERDICT: ✓ IMPROVEMENT DETECTED\n")
	default:
		fmt.Printf("  VERDICT: — NO SIGNIFICANT CHANGE\n")
	}
	fmt.Printf("  %s\n", r.VerdictDetail)
	fmt.Println()
}

// =========================================================================
// Statistical functions
// =========================================================================

// proportionZTest performs a two-proportion z-test and returns the p-value.
// Tests whether the proportion succA/nA differs significantly from succB/nB.
func proportionZTest(succA, nA, succB, nB int) float64 {
	if nA == 0 || nB == 0 {
		return 1.0 // Can't determine significance with no data
	}

	p1 := float64(succA) / float64(nA)
	p2 := float64(succB) / float64(nB)

	// Pooled proportion
	pPool := float64(succA+succB) / float64(nA+nB)

	// Standard error
	se := math.Sqrt(pPool * (1 - pPool) * (1.0/float64(nA) + 1.0/float64(nB)))
	if se == 0 {
		return 1.0
	}

	// Z statistic
	z := (p1 - p2) / se

	// Two-tailed p-value from z
	return 2 * normalCDF(-math.Abs(z))
}

// normalCDF computes the cumulative distribution function of the standard normal.
// Uses the Abramowitz and Stegun approximation.
func normalCDF(x float64) float64 {
	if x < -8 {
		return 0
	}
	if x > 8 {
		return 1
	}

	// Constants for the approximation
	const (
		a1 = 0.254829592
		a2 = -0.284496736
		a3 = 1.421413741
		a4 = -1.453152027
		a5 = 1.061405429
		p  = 0.3275911
	)

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x) / math.Sqrt(2.0)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return 0.5 * (1.0 + sign*y)
}

// =========================================================================
// Helper functions
// =========================================================================

func completionRate(r *store.SimulationResult) float64 {
	if len(r.Conversations) == 0 {
		return 0
	}
	return float64(countCompleted(r)) / float64(len(r.Conversations)) * 100
}

func countCompleted(r *store.SimulationResult) int {
	n := 0
	for _, c := range r.Conversations {
		if c.GoalResult == "completed" {
			n++
		}
	}
	return n
}

func direction(delta float64, significant bool) string {
	if !significant {
		return "unchanged"
	}
	if delta > 0 {
		return "improved"
	}
	return "regressed"
}

func compareSafety(a, b *store.SimulationResult) SafetyComparison {
	sc := SafetyComparison{}
	for _, f := range a.SafetyFindings {
		switch f.Severity {
		case "critical":
			sc.CriticalBefore++
		case "high":
			sc.HighBefore++
		case "medium":
			sc.MediumBefore++
		}
	}
	for _, f := range b.SafetyFindings {
		switch f.Severity {
		case "critical":
			sc.CriticalAfter++
		case "high":
			sc.HighAfter++
		case "medium":
			sc.MediumAfter++
		}
	}

	totalBefore := sc.CriticalBefore*3 + sc.HighBefore*2 + sc.MediumBefore
	totalAfter := sc.CriticalAfter*3 + sc.HighAfter*2 + sc.MediumAfter
	if totalAfter < totalBefore {
		sc.Direction = "improved"
	} else if totalAfter > totalBefore {
		sc.Direction = "regressed"
	} else {
		sc.Direction = "unchanged"
	}
	return sc
}

func diffFailurePatterns(a, b *store.SimulationResult) []FailurePatternDiff {
	beforeMap := map[string]int{}
	afterMap := map[string]int{}

	for _, p := range a.FailurePatterns {
		beforeMap[p.Pattern] = p.Count
	}
	// Also gather from conversations directly for runs without graded patterns
	for _, c := range a.Conversations {
		if c.GoalResult == "failed" && c.GoalReason != "" {
			beforeMap[c.GoalReason]++
		}
	}
	for _, p := range b.FailurePatterns {
		afterMap[p.Pattern] = p.Count
	}
	for _, c := range b.Conversations {
		if c.GoalResult == "failed" && c.GoalReason != "" {
			afterMap[c.GoalReason]++
		}
	}

	allPatterns := map[string]bool{}
	for p := range beforeMap {
		allPatterns[p] = true
	}
	for p := range afterMap {
		allPatterns[p] = true
	}

	var diffs []FailurePatternDiff
	for pattern := range allPatterns {
		before := beforeMap[pattern]
		after := afterMap[pattern]
		delta := after - before

		status := "unchanged"
		if before == 0 && after > 0 {
			status = "new"
		} else if before > 0 && after == 0 {
			status = "resolved"
		} else if delta > 0 {
			status = "worsened"
		} else if delta < 0 {
			status = "improved"
		}

		if status != "unchanged" {
			diffs = append(diffs, FailurePatternDiff{
				Pattern:     pattern,
				CountBefore: before,
				CountAfter:  after,
				Delta:       delta,
				Status:      status,
			})
		}
	}

	// Sort: new first, then worsened, then improved, then resolved
	statusOrder := map[string]int{"new": 0, "worsened": 1, "improved": 2, "resolved": 3}
	sort.Slice(diffs, func(i, j int) bool {
		oi, oj := statusOrder[diffs[i].Status], statusOrder[diffs[j].Status]
		if oi != oj {
			return oi < oj
		}
		return math.Abs(float64(diffs[i].Delta)) > math.Abs(float64(diffs[j].Delta))
	})

	return diffs
}

func computeVerdict(r *ComparisonResult) (string, string) {
	var reasons []string

	// Check for significant overall regression
	if r.Overall.Significant && r.Overall.Delta < 0 {
		reasons = append(reasons, fmt.Sprintf("Overall goal completion dropped %.1f%% (p=%.3f)", -r.Overall.Delta, r.Overall.PValue))
	}

	// Check for safety degradation
	if r.Safety.CriticalAfter > r.Safety.CriticalBefore {
		reasons = append(reasons, fmt.Sprintf("Critical safety findings increased (%d → %d)", r.Safety.CriticalBefore, r.Safety.CriticalAfter))
	}

	// Check for per-persona regressions
	for name, mc := range r.ByPersona {
		if mc.Significant && mc.Delta < -5 {
			reasons = append(reasons, fmt.Sprintf("%s completion dropped %.1f%% (p=%.3f)", name, -mc.Delta, mc.PValue))
		}
	}

	// Check for new failure patterns
	newPatterns := filterPatterns(r.NewFailurePatterns, "new")
	if len(newPatterns) > 0 {
		reasons = append(reasons, fmt.Sprintf("%d new failure pattern(s) detected", len(newPatterns)))
	}

	if len(reasons) > 0 {
		return "regression", strings.Join(reasons, "; ")
	}

	// Check for improvement
	if r.Overall.Significant && r.Overall.Delta > 0 {
		return "improvement", fmt.Sprintf("Overall goal completion improved %.1f%% (p=%.3f)", r.Overall.Delta, r.Overall.PValue)
	}
	if r.Safety.Direction == "improved" {
		return "improvement", "Safety findings decreased"
	}

	return "no_change", "No statistically significant differences detected"
}

func filterPatterns(patterns []FailurePatternDiff, statuses ...string) []FailurePatternDiff {
	statusSet := map[string]bool{}
	for _, s := range statuses {
		statusSet[s] = true
	}
	var out []FailurePatternDiff
	for _, p := range patterns {
		if statusSet[p.Status] {
			out = append(out, p)
		}
	}
	return out
}

func printMetric(label string, m MetricComparison) {
	indicator := "▲"
	if m.Delta < 0 {
		indicator = "▼"
	}
	fmt.Printf("%-24s %.1f%% → %.1f%%  [%s %+.1f%%]", label, m.Before, m.After, indicator, m.Delta)
	if m.Significant {
		if m.Delta < 0 {
			fmt.Printf(" ⚠ REGRESSION (p=%.3f)", m.PValue)
		} else {
			fmt.Printf(" ✓ IMPROVED (p=%.3f)", m.PValue)
		}
	} else if m.PValue < 1.0 {
		fmt.Printf(" (p=%.3f, not significant)", m.PValue)
	}
	fmt.Println()
}

func printSafetyDirection(delta int) {
	if delta < 0 {
		fmt.Print(" ✓ IMPROVED")
	} else if delta > 0 {
		fmt.Print(" ⚠ DEGRADED")
	}
}
