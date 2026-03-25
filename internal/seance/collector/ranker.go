// Smart context selection for Seance — ranks sources by relevance to the
// question and allocates a token budget to control context size.
package collector

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/seance/connector"
)

// DefaultTokenBudget is the maximum tokens to allocate across all sources.
const DefaultTokenBudget = 4000

// approxTokensPerChar is a rough estimate of tokens per character for English text.
const approxTokensPerChar = 0.25

// RankedSource holds a source with its computed relevance score.
type RankedSource struct {
	Source connector.Source
	Score  float64
	Reason string // why this score was assigned
}

// RankSources scores each source's relevance to the given question using
// keyword overlap and source type matching.
func RankSources(question string, sources []connector.Source) []RankedSource {
	questionLower := strings.ToLower(question)
	keywords := extractKeywords(questionLower)

	var ranked []RankedSource
	for _, src := range sources {
		score, reason := scoreSource(questionLower, keywords, src)
		ranked = append(ranked, RankedSource{
			Source: src,
			Score:  score,
			Reason: reason,
		})
	}

	// Sort by score descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

// scoreSource computes a relevance score (0.0 to 1.0) for a source.
func scoreSource(questionLower string, keywords []string, src connector.Source) (float64, string) {
	score := 0.0
	var reasons []string

	srcName := strings.ToLower(src.Name())
	srcType := strings.ToLower(src.Type())

	// 1. Name overlap: does the source name appear in the question?
	if strings.Contains(questionLower, srcName) {
		score += 0.3
		reasons = append(reasons, "name match")
	}

	// 2. Keyword overlap with source name
	nameKeywords := strings.Fields(srcName)
	for _, nk := range nameKeywords {
		for _, qk := range keywords {
			if nk == qk || strings.Contains(nk, qk) || strings.Contains(qk, nk) {
				score += 0.15
				reasons = append(reasons, "keyword: "+qk)
			}
		}
	}

	// 3. Source type matching based on question topic
	typeScore, typeReason := matchTypeToQuestion(questionLower, keywords, srcType)
	score += typeScore
	if typeReason != "" {
		reasons = append(reasons, typeReason)
	}

	// 4. Baseline score: every source gets a minimum relevance
	score += 0.1

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}

	reason := strings.Join(reasons, "; ")
	if reason == "" {
		reason = "baseline"
	}
	return score, reason
}

// matchTypeToQuestion assigns a score boost based on source type vs. question content.
func matchTypeToQuestion(question string, keywords []string, srcType string) (float64, string) {
	// Topic -> source type affinity map
	topicTypes := map[string][]string{
		"error":      {"log", "health", "http", "prometheus"},
		"slow":       {"prometheus", "http", "health"},
		"latency":    {"prometheus", "http"},
		"memory":     {"prometheus", "env"},
		"cpu":        {"prometheus"},
		"disk":       {"prometheus", "env"},
		"database":   {"sql", "redis", "health"},
		"db":         {"sql", "redis", "health"},
		"cache":      {"redis"},
		"redis":      {"redis"},
		"postgres":   {"sql"},
		"mysql":      {"sql"},
		"log":        {"log"},
		"crash":      {"log", "health"},
		"deploy":     {"env", "health", "log"},
		"config":     {"env", "static"},
		"health":     {"health", "http"},
		"status":     {"health", "http", "prometheus"},
		"metric":     {"prometheus"},
		"request":    {"prometheus", "http", "log"},
		"traffic":    {"prometheus", "http"},
		"queue":      {"redis", "prometheus"},
		"connection": {"sql", "redis", "health"},
		"timeout":    {"http", "health", "log", "prometheus"},
	}

	bestScore := 0.0
	bestReason := ""

	for _, kw := range keywords {
		if affinityTypes, ok := topicTypes[kw]; ok {
			for i, at := range affinityTypes {
				if at == srcType {
					// Higher score for primary match, lower for secondary
					s := 0.3 - float64(i)*0.05
					if s < 0.05 {
						s = 0.05
					}
					if s > bestScore {
						bestScore = s
						bestReason = "type affinity: " + kw + "->" + srcType
					}
				}
			}
		}
	}

	// Also check full question for common patterns
	patterns := map[string]map[string]float64{
		"prometheus": {"rate": 0.2, "p99": 0.25, "p95": 0.25, "percentile": 0.2, "metric": 0.2},
		"log":        {"error": 0.15, "exception": 0.2, "stack": 0.2, "trace": 0.15},
		"sql":        {"query": 0.15, "table": 0.2, "row": 0.15, "column": 0.15},
		"redis":      {"key": 0.1, "cache": 0.2, "ttl": 0.15, "expire": 0.15},
		"health":     {"healthy": 0.2, "alive": 0.2, "up": 0.1, "down": 0.2},
	}

	if typePatterns, ok := patterns[srcType]; ok {
		for pattern, s := range typePatterns {
			if strings.Contains(question, pattern) && s > bestScore {
				bestScore = s
				bestReason = "pattern: " + pattern + " -> " + srcType
			}
		}
	}

	return bestScore, bestReason
}

// extractKeywords pulls significant words from a question.
func extractKeywords(question string) []string {
	stopWords := map[string]bool{
		"why": true, "is": true, "the": true, "a": true, "an": true,
		"what": true, "how": true, "when": true, "where": true, "who": true,
		"are": true, "was": true, "were": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"can": true, "may": true, "might": true, "shall": true, "must": true,
		"my": true, "your": true, "our": true, "their": true, "its": true,
		"this": true, "that": true, "these": true, "those": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"with": true, "from": true, "of": true, "about": true, "into": true,
		"not": true, "so": true, "up": true, "out": true, "if": true,
		"and": true, "or": true, "but": true, "than": true, "then": true,
		"right": true, "now": true, "just": true, "very": true,
	}

	words := strings.Fields(strings.ToLower(question))
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, "?.!,;:'\"")
		if len(w) >= 2 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// AllocateTokenBudget distributes a token budget across ranked sources
// proportional to their relevance scores. Returns a map of source name
// to max token allocation.
func AllocateTokenBudget(ranked []RankedSource, totalBudget int) map[string]int {
	if totalBudget <= 0 {
		totalBudget = DefaultTokenBudget
	}

	allocations := map[string]int{}
	if len(ranked) == 0 {
		return allocations
	}

	// Calculate total score
	totalScore := 0.0
	for _, rs := range ranked {
		totalScore += rs.Score
	}
	if totalScore == 0 {
		totalScore = float64(len(ranked)) * 0.1 // avoid division by zero
	}

	// Allocate proportionally with a minimum floor
	minTokens := 100
	remaining := totalBudget

	for _, rs := range ranked {
		proportion := rs.Score / totalScore
		tokens := int(math.Round(float64(totalBudget) * proportion))

		// Enforce minimum
		if tokens < minTokens {
			tokens = minTokens
		}
		if tokens > remaining {
			tokens = remaining
		}

		allocations[rs.Source.Name()] = tokens
		remaining -= tokens
		if remaining <= 0 {
			break
		}
	}

	return allocations
}

// TruncateToFit performs smart truncation of data to fit within a token budget.
// It preserves key information like error lines, numbers, and the beginning/end.
func TruncateToFit(data string, maxTokens int) string {
	if maxTokens <= 0 {
		maxTokens = DefaultTokenBudget
	}

	maxChars := int(float64(maxTokens) / approxTokensPerChar)

	if len(data) <= maxChars {
		return data
	}

	lines := strings.Split(data, "\n")
	if len(lines) <= 3 {
		// Short content: just hard-truncate
		return data[:maxChars] + "\n...[truncated]"
	}

	// Score each line for importance
	type scoredLine struct {
		text  string
		idx   int
		score float64
	}

	scored := make([]scoredLine, len(lines))
	for i, line := range lines {
		scored[i] = scoredLine{text: line, idx: i, score: scoreLine(line, i, len(lines))}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Select lines within budget
	var selected []scoredLine
	charCount := 0
	for _, sl := range scored {
		lineChars := len(sl.text) + 1 // +1 for newline
		if charCount+lineChars > maxChars {
			continue
		}
		selected = append(selected, sl)
		charCount += lineChars
	}

	// Re-sort by original position
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].idx < selected[j].idx
	})

	var result strings.Builder
	prevIdx := -1
	for _, sl := range selected {
		if prevIdx >= 0 && sl.idx > prevIdx+1 {
			result.WriteString("...\n")
		}
		result.WriteString(sl.text)
		result.WriteString("\n")
		prevIdx = sl.idx
	}

	if len(selected) < len(lines) {
		result.WriteString(strings.Repeat(".", 3))
		result.WriteString("[truncated: kept ")
		result.WriteString(strings.TrimRight(strings.TrimRight(
			strings.Replace(
				strings.Replace(
					fmt.Sprintf("%.0f", float64(len(selected))/float64(len(lines))*100),
					".000000", "", 1),
				".0", "", 1),
			"0"), "."))
		result.WriteString("% of lines]\n")
	}

	return result.String()
}

// scoreLine assigns an importance score to a line for truncation decisions.
func scoreLine(line string, idx, total int) float64 {
	score := 0.0
	lower := strings.ToLower(line)

	// Headers and separators are important
	if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "###") {
		score += 3.0
	}

	// Error lines are high priority
	if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
		strings.Contains(lower, "critical") || strings.Contains(lower, "fatal") {
		score += 5.0
	}

	// Warning lines
	if strings.Contains(lower, "warn") || strings.Contains(lower, "timeout") {
		score += 3.0
	}

	// Lines with numbers/metrics are valuable
	for _, c := range line {
		if c >= '0' && c <= '9' {
			score += 0.5
			break
		}
	}

	// Status indicators
	if strings.Contains(lower, "status") || strings.Contains(lower, "healthy") ||
		strings.Contains(lower, "unhealthy") || strings.Contains(lower, "down") {
		score += 2.0
	}

	// First and last lines get a boost (context anchors)
	if idx < 3 {
		score += 2.0 - float64(idx)*0.5
	}
	if idx >= total-3 {
		score += 2.0 - float64(total-1-idx)*0.5
	}

	// Empty lines are low priority
	if strings.TrimSpace(line) == "" {
		score -= 2.0
	}

	return score
}

