// Package suggest uses an LLM to prioritize gaps and generate actionable recommendations.
package suggest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Recommendation is an LLM-prioritized suggestion for addressing a gap.
type Recommendation struct {
	Priority    int    `json:"priority"`     // 1 = most important
	Gap         gaps.Gap `json:"gap"`
	Impact      string `json:"impact"`       // what happens if you don't fix this
	Effort      string `json:"effort"`       // low, medium, high
	FixSketch   string `json:"fix_sketch"`   // rough code outline
}

// Engine generates prioritized recommendations.
type Engine struct {
	llm   provider.Provider
	model string
}

// New creates a suggestion engine.
func New(llm provider.Provider, model string) *Engine {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Engine{llm: llm, model: model}
}

// Prioritize takes gaps and returns prioritized recommendations.
func (e *Engine) Prioritize(ctx context.Context, gapList []gaps.Gap, maxRecs int) ([]Recommendation, error) {
	if maxRecs <= 0 {
		maxRecs = 10
	}
	if len(gapList) == 0 {
		return nil, nil
	}

	// Take top gaps by severity for LLM analysis
	top := gapList
	if len(top) > 30 {
		top = top[:30]
	}

	prompt := buildPrioritizePrompt(top, maxRecs)

	temp := 0.2
	maxTokens := 2000
	resp, err := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var recs []Recommendation
	if err := json.Unmarshal([]byte(raw), &recs); err != nil {
		// Fallback: return gaps sorted by severity
		return fallbackRecommendations(gapList, maxRecs), nil
	}

	// Attach original gap data
	gapMap := map[string]gaps.Gap{}
	for _, g := range gapList {
		gapMap[g.ID] = g
	}
	for i := range recs {
		if g, ok := gapMap[recs[i].Gap.ID]; ok {
			recs[i].Gap = g
		}
		recs[i].Priority = i + 1
	}

	return recs, nil
}

func buildPrioritizePrompt(gapList []gaps.Gap, max int) string {
	var sb strings.Builder
	sb.WriteString("You are a software quality expert. Prioritize these code gaps by risk and impact.\n\n")
	sb.WriteString(fmt.Sprintf("Return a JSON array of the top %d most important gaps to fix, in priority order.\n", max))
	sb.WriteString("Each item: {\"gap\":{\"id\":\"...\"},\"impact\":\"what happens if unfixed\",\"effort\":\"low|medium|high\",\"fix_sketch\":\"rough fix\"}\n\n")
	sb.WriteString("GAPS:\n")

	for _, g := range gapList {
		sb.WriteString(fmt.Sprintf("  ID: %s\n  Type: %s | Severity: %s | File: %s:%d\n  %s\n  Evidence: %s\n\n",
			g.ID, g.Type, g.Severity, g.File, g.Line, g.Description, truncate(g.Evidence, 100)))
	}

	return sb.String()
}

func fallbackRecommendations(gapList []gaps.Gap, max int) []Recommendation {
	sevOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

	// Sort by severity
	sorted := make([]gaps.Gap, len(gapList))
	copy(sorted, gapList)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sevOrder[sorted[j].Severity] < sevOrder[sorted[i].Severity] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if len(sorted) > max {
		sorted = sorted[:max]
	}

	var recs []Recommendation
	for i, g := range sorted {
		recs = append(recs, Recommendation{
			Priority:  i + 1,
			Gap:       g,
			Impact:    "Potential production issue if not addressed",
			Effort:    "medium",
			FixSketch: g.Suggestion,
		})
	}
	return recs
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
