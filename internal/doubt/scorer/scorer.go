// Package scorer computes confidence scores for code units based on production reality.
//
// Confidence is NOT code quality. It's "how much should I trust this code right now?"
// based on what's actually happening in production.
//
// Scoring dimensions:
//   - Volume: how much traffic has this code seen? (more = higher confidence)
//   - Success: what's the error rate? (lower = higher confidence)
//   - Stability: how long has this code been running without changes? (longer = higher)
//   - Freshness: when was this code last exercised? (recently = higher)
//   - Severity: any panics or timeouts? (none = higher)
package scorer

import (
	"math"
	"sort"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
)

// Score is the confidence assessment for a single code unit.
type Score struct {
	UnitID      string  `json:"unit_id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	File        string  `json:"file"`
	Line        int     `json:"line"`
	Confidence  float64 `json:"confidence"`   // 0.0-1.0 overall
	Volume      float64 `json:"volume"`       // 0.0-1.0
	Success     float64 `json:"success"`      // 0.0-1.0
	Stability   float64 `json:"stability"`    // 0.0-1.0
	Freshness   float64 `json:"freshness"`    // 0.0-1.0
	Severity    float64 `json:"severity"`     // 0.0-1.0
	Grade       string  `json:"grade"`        // A, B, C, D, F, ?
	Reason      string  `json:"reason"`       // human-readable explanation
	HasData     bool    `json:"has_data"`     // whether production data exists
}

// Report holds scored results for an entire codebase.
type Report struct {
	Scores       []Score        `json:"scores"`
	Summary      Summary        `json:"summary"`
	ByFile       map[string][]Score `json:"by_file"`
	ByGrade      map[string]int `json:"by_grade"`
	Generated    time.Time      `json:"generated"`
}

// Summary holds aggregate confidence statistics.
type Summary struct {
	TotalUnits       int     `json:"total_units"`
	WithData         int     `json:"with_data"`
	WithoutData      int     `json:"without_data"`
	AvgConfidence    float64 `json:"avg_confidence"`
	MedianConfidence float64 `json:"median_confidence"`
	LowestFile       string  `json:"lowest_file"`
	LowestScore      float64 `json:"lowest_score"`
	HighestFile      string  `json:"highest_file"`
	HighestScore     float64 `json:"highest_score"`
}

// Compute scores all code units against their production data.
func Compute(units []scanner.Unit, store *tracker.Store) *Report {
	allStats := store.AllStats()
	report := &Report{
		ByFile:    map[string][]Score{},
		ByGrade:   map[string]int{},
		Generated: time.Now(),
	}

	var confidences []float64

	for _, unit := range units {
		stats := allStats[unit.ID]
		score := scoreUnit(unit, stats)

		report.Scores = append(report.Scores, score)
		report.ByFile[unit.File] = append(report.ByFile[unit.File], score)
		report.ByGrade[score.Grade]++

		if score.HasData {
			confidences = append(confidences, score.Confidence)
		}
	}

	// Sort by confidence ascending (least confident first)
	sort.Slice(report.Scores, func(i, j int) bool {
		return report.Scores[i].Confidence < report.Scores[j].Confidence
	})

	// Compute summary
	report.Summary = computeSummary(report.Scores, confidences)

	return report
}

func scoreUnit(unit scanner.Unit, stats *tracker.UnitStats) Score {
	s := Score{
		UnitID: unit.ID,
		Name:   unit.Name,
		Type:   unit.Type,
		File:   unit.File,
		Line:   unit.Line,
	}

	if stats == nil {
		// No production data — this is the scariest state
		s.Confidence = 0
		s.Grade = "?"
		s.Reason = "no production data — never exercised in production"
		s.HasData = false
		return s
	}

	s.HasData = true

	// Volume: how much traffic? (log scale, saturates around 10K requests)
	s.Volume = clamp(math.Log10(float64(stats.Requests)+1)/4, 0, 1)

	// Success: inverse of error rate
	if stats.Requests > 0 {
		s.Success = clamp(1-(stats.ErrorRate/100), 0, 1)
	} else {
		s.Success = 0.5 // no data
	}

	// Stability: how long has this code been running? (saturates around 30 days)
	s.Stability = clamp(stats.DaysSinceFirst/30, 0, 1)

	// Freshness: was this code exercised recently? (decays over 7 days)
	daysSinceLastSeen := time.Since(stats.LastSeen).Hours() / 24
	s.Freshness = clamp(1-(daysSinceLastSeen/7), 0, 1)

	// Severity: panics and timeouts are very bad signals
	if stats.Panics > 0 {
		s.Severity = 0 // any panic = zero severity confidence
	} else if stats.Timeouts > 0 {
		s.Severity = clamp(1-float64(stats.Timeouts)/float64(stats.Requests), 0, 1)
	} else {
		s.Severity = 1.0 // no panics or timeouts = full severity confidence
	}

	// Weighted combination
	s.Confidence = s.Volume*0.20 + s.Success*0.30 + s.Stability*0.15 + s.Freshness*0.15 + s.Severity*0.20

	// Grade
	switch {
	case s.Confidence >= 0.9:
		s.Grade = "A"
	case s.Confidence >= 0.75:
		s.Grade = "B"
	case s.Confidence >= 0.6:
		s.Grade = "C"
	case s.Confidence >= 0.4:
		s.Grade = "D"
	default:
		s.Grade = "F"
	}

	// Human-readable reason
	s.Reason = buildReason(s, stats)

	return s
}

func buildReason(s Score, stats *tracker.UnitStats) string {
	var concerns []string

	if stats.Requests < 10 {
		concerns = append(concerns, "very low traffic")
	} else if stats.Requests < 100 {
		concerns = append(concerns, "low traffic")
	}

	if stats.ErrorRate > 5 {
		concerns = append(concerns, "high error rate")
	} else if stats.ErrorRate > 1 {
		concerns = append(concerns, "elevated error rate")
	}

	if stats.Panics > 0 {
		concerns = append(concerns, "has panics in production")
	}

	if stats.Timeouts > 0 {
		concerns = append(concerns, "has timeouts")
	}

	if stats.DaysSinceFirst < 1 {
		concerns = append(concerns, "deployed less than 24h ago")
	}

	daysSinceLastSeen := time.Since(stats.LastSeen).Hours() / 24
	if daysSinceLastSeen > 3 {
		concerns = append(concerns, "not exercised recently")
	}

	if len(concerns) == 0 {
		return "healthy — good traffic, low errors, stable"
	}
	return joinConcerns(concerns)
}

func joinConcerns(concerns []string) string {
	if len(concerns) == 1 {
		return concerns[0]
	}
	return concerns[0] + "; " + joinConcerns(concerns[1:])
}

func computeSummary(scores []Score, confidences []float64) Summary {
	s := Summary{
		TotalUnits: len(scores),
	}

	for _, sc := range scores {
		if sc.HasData {
			s.WithData++
		} else {
			s.WithoutData++
		}
	}

	if len(confidences) > 0 {
		var sum float64
		for _, c := range confidences {
			sum += c
		}
		s.AvgConfidence = sum / float64(len(confidences))

		sorted := make([]float64, len(confidences))
		copy(sorted, confidences)
		sort.Float64s(sorted)
		s.MedianConfidence = sorted[len(sorted)/2]
	}

	// Find lowest and highest scored files
	fileScores := map[string][]float64{}
	for _, sc := range scores {
		if sc.HasData {
			fileScores[sc.File] = append(fileScores[sc.File], sc.Confidence)
		}
	}

	lowestAvg := 2.0
	highestAvg := -1.0
	for file, scores := range fileScores {
		var sum float64
		for _, c := range scores {
			sum += c
		}
		avg := sum / float64(len(scores))
		if avg < lowestAvg {
			lowestAvg = avg
			s.LowestFile = file
			s.LowestScore = avg
		}
		if avg > highestAvg {
			highestAvg = avg
			s.HighestFile = file
			s.HighestScore = avg
		}
	}

	return s
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
