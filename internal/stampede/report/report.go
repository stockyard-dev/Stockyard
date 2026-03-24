// Package report generates human-readable reports from simulation results.
package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/stampede/store"
)

// Terminal prints a formatted report to stdout.
func Terminal(r *store.SimulationResult) {
	if r == nil {
		fmt.Println("  No results available.")
		return
	}

	totalConvs := len(r.Conversations)
	totalCompleted := 0
	totalPartial := 0
	totalFailed := 0
	totalErrors := 0

	for _, c := range r.Conversations {
		switch c.GoalResult {
		case "completed":
			totalCompleted++
		case "partial":
			totalPartial++
		case "failed":
			totalFailed++
		}
		if c.Status == "error" {
			totalErrors++
		}
	}

	overallRate := 0.0
	if totalConvs > 0 {
		overallRate = float64(totalCompleted) / float64(totalConvs) * 100
	}

	fmt.Println()
	fmt.Println("  ═══ STAMPEDE REPORT ══════════════════════════════════════════")
	fmt.Printf("  Target:   %s\n", r.TargetURL)
	fmt.Printf("  Duration: %s  |  Users: %d  |  Conversations: %d\n",
		r.Duration.Round(1e9), r.UserCount, totalConvs)
	fmt.Printf("  Cost:     $%.4f\n", r.TotalCost)
	fmt.Println()

	// Goal completion table
	fmt.Println("  GOAL COMPLETION")
	fmt.Printf("  %-22s %6s %10s %9s %8s %7s\n", "Persona", "Total", "Completed", "Partial", "Failed", "Rate")
	fmt.Printf("  %-22s %6s %10s %9s %8s %7s\n",
		strings.Repeat("─", 22), strings.Repeat("─", 6), strings.Repeat("─", 10),
		strings.Repeat("─", 9), strings.Repeat("─", 8), strings.Repeat("─", 7))

	// Sort persona names for consistent output
	var personaNames []string
	for name := range r.PersonaBreakdown {
		personaNames = append(personaNames, name)
	}
	sort.Strings(personaNames)

	for _, name := range personaNames {
		ps := r.PersonaBreakdown[name]
		fmt.Printf("  %-22s %6d %10d %9d %8d %6.1f%%\n",
			name, ps.Total, ps.Completed, ps.Partial, ps.Failed, ps.Rate)
	}
	fmt.Printf("  %-22s %6d %10d %9d %8d %6.1f%%\n",
		"OVERALL", totalConvs, totalCompleted, totalPartial, totalFailed, overallRate)

	if totalErrors > 0 {
		fmt.Printf("\n  ⚠ %d conversations encountered errors\n", totalErrors)
	}

	// Graded results (if analysis ran, these override self-reported)
	if len(r.Grades) > 0 {
		gradedCompleted := 0
		gradedPartial := 0
		gradedFailed := 0
		for _, g := range r.Grades {
			switch g.Result {
			case "completed":
				gradedCompleted++
			case "partial":
				gradedPartial++
			case "failed":
				gradedFailed++
			}
		}
		gradedRate := 0.0
		if len(r.Grades) > 0 {
			gradedRate = float64(gradedCompleted) / float64(len(r.Grades)) * 100
		}
		fmt.Println()
		fmt.Println("  GRADED RESULTS (LLM judge)")
		fmt.Printf("  Completed: %d  |  Partial: %d  |  Failed: %d  |  Rate: %.1f%%\n",
			gradedCompleted, gradedPartial, gradedFailed, gradedRate)
	}

	// Safety findings
	if len(r.SafetyFindings) > 0 {
		critical, high, medium, low := 0, 0, 0, 0
		for _, f := range r.SafetyFindings {
			switch f.Severity {
			case "critical":
				critical++
			case "high":
				high++
			case "medium":
				medium++
			case "low":
				low++
			}
		}
		fmt.Println()
		fmt.Println("  ⚠  SAFETY FINDINGS")
		if critical > 0 {
			fmt.Printf("  │ %d CRITICAL\n", critical)
			for _, f := range r.SafetyFindings {
				if f.Severity == "critical" {
					fmt.Printf("  │   → %s (conv %s, turn %d)\n", f.Description, f.ConversationID, f.TurnNumber)
				}
			}
		}
		if high > 0 {
			fmt.Printf("  │ %d HIGH\n", high)
			for _, f := range r.SafetyFindings {
				if f.Severity == "high" {
					fmt.Printf("  │   → %s (conv %s, turn %d)\n", f.Description, f.ConversationID, f.TurnNumber)
				}
			}
		}
		if medium > 0 {
			fmt.Printf("  │ %d MEDIUM\n", medium)
		}
		if low > 0 {
			fmt.Printf("  │ %d LOW\n", low)
		}
	} else {
		fmt.Println()
		fmt.Println("  ✓ SAFETY: No findings detected")
	}

	// Top failure patterns (from graded results)
	if len(r.FailurePatterns) > 0 {
		fmt.Println()
		fmt.Println("  TOP FAILURE PATTERNS")
		limit := 5
		if len(r.FailurePatterns) < limit {
			limit = len(r.FailurePatterns)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  │ %d. %s (%d conversations)\n",
				i+1, r.FailurePatterns[i].Pattern, r.FailurePatterns[i].Count)
		}
	}

	// Top suggestions from grader
	suggestions := collectSuggestions(r)
	if len(suggestions) > 0 {
		fmt.Println()
		fmt.Println("  IMPROVEMENT SUGGESTIONS (from grader)")
		limit := 3
		if len(suggestions) < limit {
			limit = len(suggestions)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  │ • %s\n", suggestions[i])
		}
	}

	fmt.Println()
	fmt.Printf("  Simulation ID: %s\n", r.ID)
	fmt.Printf("  Full report:   stampede report %s --html > report.html\n", r.ID)
	fmt.Println()
}

// HTML generates a self-contained HTML report.
func HTML(r *store.SimulationResult) string {
	data, _ := json.MarshalIndent(r, "", "  ")

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Stampede Report — %s</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: Georgia, serif; background: #1a1510; color: #f5f0e8; padding: 2rem; }
h1 { font-family: 'Libre Baskerville', Georgia, serif; color: #d4a574; margin-bottom: 0.5rem; }
h2 { font-family: 'Libre Baskerville', Georgia, serif; color: #8b4513; margin: 2rem 0 1rem; }
.meta { color: #8a7e6e; font-size: 0.9rem; margin-bottom: 2rem; }
table { width: 100%%; border-collapse: collapse; margin: 1rem 0; }
th { background: #3e2723; color: #fff8f0; padding: 0.6rem 1rem; text-align: left; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; }
td { padding: 0.5rem 1rem; border-bottom: 1px solid #3a3228; font-size: 0.9rem; }
tr:hover td { background: #2a2318; }
.rate { font-weight: bold; }
.good { color: #66bb6a; }
.warn { color: #ffa726; }
.bad { color: #ef5350; }
.card { background: #2a2318; border-radius: 8px; padding: 1.5rem; margin: 1rem 0; }
.stat { display: inline-block; margin-right: 2rem; }
.stat-value { font-size: 1.8rem; font-weight: bold; color: #d4a574; }
.stat-label { font-size: 0.8rem; color: #8a7e6e; }
pre { background: #111; padding: 1rem; border-radius: 4px; overflow-x: auto; font-size: 0.8rem; color: #ccc; }
</style>
</head>
<body>
<h1>🐂 Stampede Report</h1>
<p class="meta">Simulation %s — %s — %d users — %d conversations</p>

<div class="card">
  <div class="stat"><div class="stat-value">%d</div><div class="stat-label">Conversations</div></div>
  <div class="stat"><div class="stat-value">%.1f%%%%</div><div class="stat-label">Goal Completion</div></div>
  <div class="stat"><div class="stat-value">$%.4f</div><div class="stat-label">Total Cost</div></div>
  <div class="stat"><div class="stat-value">%s</div><div class="stat-label">Duration</div></div>
</div>

<h2>Results by Persona</h2>
<table>
<tr><th>Persona</th><th>Total</th><th>Completed</th><th>Partial</th><th>Failed</th><th>Rate</th></tr>
%s
</table>

<h2>Raw Data</h2>
<pre>%s</pre>

<p style="color:#5a5248;margin-top:2rem;text-align:center;">Generated by Stockyard Stampede · stockyard.dev/stampede</p>
</body>
</html>`,
		r.ID,
		r.ID, r.TargetURL, r.UserCount, len(r.Conversations),
		len(r.Conversations), overallRate(r), r.TotalCost, r.Duration.Round(1e9),
		personaRows(r),
		string(data),
	)
}

// Compare prints a regression comparison between two simulation runs.
func Compare(a, b *store.SimulationResult) {
	fmt.Println()
	fmt.Println("  ═══ REGRESSION REPORT ═════════════════════════════════════════")
	fmt.Printf("  Comparing: %s vs %s\n", a.ID, b.ID)
	fmt.Println()

	rateA := overallRate(a)
	rateB := overallRate(b)
	delta := rateB - rateA

	indicator := "▲"
	if delta < 0 {
		indicator = "▼"
	}

	fmt.Printf("  GOAL COMPLETION (overall)\n")
	fmt.Printf("    Before: %.1f%%  →  After: %.1f%%  [%s %+.1f%%]", rateA, rateB, indicator, delta)
	if delta < -3 {
		fmt.Print(" ⚠ REGRESSION")
	} else if delta > 3 {
		fmt.Print(" ✓ IMPROVED")
	}
	fmt.Println()

	// Per-persona comparison
	fmt.Println()
	fmt.Println("  GOAL COMPLETION (by persona)")
	allNames := map[string]bool{}
	for n := range a.PersonaBreakdown {
		allNames[n] = true
	}
	for n := range b.PersonaBreakdown {
		allNames[n] = true
	}

	var names []string
	for n := range allNames {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		pa := a.PersonaBreakdown[name]
		pb := b.PersonaBreakdown[name]
		d := pb.Rate - pa.Rate
		ind := "▲"
		if d < 0 {
			ind = "▼"
		}
		suffix := ""
		if d < -5 {
			suffix = " ⚠ REGRESSION"
		}
		fmt.Printf("    %-22s %.1f%% → %.1f%%  [%s %+.1f%%]%s\n",
			name, pa.Rate, pb.Rate, ind, d, suffix)
	}

	fmt.Println()
	if delta < -3 {
		fmt.Println("  VERDICT: REGRESSION DETECTED")
	} else if delta > 3 {
		fmt.Println("  VERDICT: IMPROVEMENT DETECTED")
	} else {
		fmt.Println("  VERDICT: NO SIGNIFICANT CHANGE")
	}

	// Safety comparison
	safetyCritA, safetyCritB := 0, 0
	for _, f := range a.SafetyFindings {
		if f.Severity == "critical" || f.Severity == "high" {
			safetyCritA++
		}
	}
	for _, f := range b.SafetyFindings {
		if f.Severity == "critical" || f.Severity == "high" {
			safetyCritB++
		}
	}
	if safetyCritA > 0 || safetyCritB > 0 {
		fmt.Println()
		fmt.Printf("  SAFETY (critical+high): %d → %d", safetyCritA, safetyCritB)
		if safetyCritB < safetyCritA {
			fmt.Print(" ✓ IMPROVED")
		} else if safetyCritB > safetyCritA {
			fmt.Print(" ⚠ DEGRADED")
		}
		fmt.Println()
	}

	fmt.Println()
}

type failureReason struct {
	Reason string
	Count  int
}

func collectFailureReasons(r *store.SimulationResult) []failureReason {
	counts := map[string]int{}
	for _, c := range r.Conversations {
		if c.GoalResult == "failed" && c.GoalReason != "" {
			counts[c.GoalReason]++
		}
	}

	var out []failureReason
	for reason, count := range counts {
		out = append(out, failureReason{Reason: reason, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func overallRate(r *store.SimulationResult) float64 {
	if len(r.Conversations) == 0 {
		return 0
	}
	completed := 0
	for _, c := range r.Conversations {
		if c.GoalResult == "completed" {
			completed++
		}
	}
	return float64(completed) / float64(len(r.Conversations)) * 100
}

func personaRows(r *store.SimulationResult) string {
	var names []string
	for n := range r.PersonaBreakdown {
		names = append(names, n)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		ps := r.PersonaBreakdown[name]
		rateClass := "good"
		if ps.Rate < 50 {
			rateClass = "bad"
		} else if ps.Rate < 75 {
			rateClass = "warn"
		}
		sb.WriteString(fmt.Sprintf(
			"<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td class=\"rate %s\">%.1f%%</td></tr>\n",
			name, ps.Total, ps.Completed, ps.Partial, ps.Failed, rateClass, ps.Rate,
		))
	}
	return sb.String()
}

// collectSuggestions extracts unique improvement suggestions from grades.
func collectSuggestions(r *store.SimulationResult) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range r.Grades {
		if g.Suggestion != "" && !seen[g.Suggestion] {
			seen[g.Suggestion] = true
			out = append(out, g.Suggestion)
		}
	}
	return out
}
