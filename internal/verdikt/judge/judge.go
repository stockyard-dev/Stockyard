// Package judge evaluates AI responses independently. It watches every response
// your AI produces and scores it — not by rules, but by learning what "good"
// looks like for YOUR specific use case.
package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/verdikt/rubric"
	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

// Evaluation is the judge's assessment of a single AI response.
type Evaluation struct {
	ID           string    `json:"id"`
	RequestID    string    `json:"request_id"`
	Score        float64   `json:"score"`       // 0-1 overall quality
	Dimensions   Scores    `json:"dimensions"`
	Verdict      string    `json:"verdict"`     // good, acceptable, poor, bad
	Issues       []string  `json:"issues,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	LatencyMs    int       `json:"latency_ms"`
}

// Scores breaks quality into dimensions.
type Scores struct {
	Relevance   float64 `json:"relevance"`    // did it answer the question?
	Accuracy    float64 `json:"accuracy"`     // is the answer correct?
	Helpfulness float64 `json:"helpfulness"`  // is it actually useful?
	Safety      float64 `json:"safety"`       // no harmful content?
	Tone        float64 `json:"tone"`         // appropriate tone for context?
}

// Interaction represents an AI request-response pair to evaluate.
type Interaction struct {
	RequestID    string         `json:"request_id"`
	Prompt       string         `json:"prompt"`
	Response     string         `json:"response"`
	Model        string         `json:"model"`
	Context      string         `json:"context,omitempty"`   // system prompt or conversation context
	Domain       string         `json:"domain,omitempty"`    // customer-support, coding, medical, etc.
	UserReaction string         `json:"user_reaction,omitempty"` // accepted, rephrased, escalated, abandoned
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Engine evaluates AI interactions.
type Engine struct {
	llm       provider.Provider
	model     string
	domain    string
	db        *store.DB
	mu        sync.RWMutex
	history   []Evaluation
	baselines map[string]float64 // learned quality baselines per domain
	rubrics   map[string]*rubric.Rubric
}

// New creates a judge engine. If db is non-nil, evaluations are persisted
// and baselines are loaded from SQLite on startup.
func New(llm provider.Provider, model, domain string, db *store.DB) *Engine {
	if model == "" {
		model = "gpt-4o-mini"
	}
	e := &Engine{
		llm:       llm,
		model:     model,
		domain:    domain,
		db:        db,
		baselines: map[string]float64{},
		rubrics:   map[string]*rubric.Rubric{"default": rubric.DefaultRubric()},
	}
	if db != nil {
		e.loadFromDB()
	}
	return e
}

// SetRubrics replaces the loaded rubric set.
func (e *Engine) SetRubrics(rubrics map[string]*rubric.Rubric) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rubrics = rubrics
	// Always keep the default
	if _, ok := e.rubrics["default"]; !ok {
		e.rubrics["default"] = rubric.DefaultRubric()
	}
}

// Rubrics returns the currently loaded rubrics.
func (e *Engine) Rubrics() map[string]*rubric.Rubric {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rubrics
}

// rubricFor returns the rubric matching the domain, falling back to default.
func (e *Engine) rubricFor(domain string) *rubric.Rubric {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if r, ok := e.rubrics[domain]; ok {
		return r
	}
	return e.rubrics["default"]
}

// loadFromDB restores in-memory state from the database.
func (e *Engine) loadFromDB() {
	baselines, err := e.db.ListBaselines()
	if err != nil {
		log.Printf("verdikt/judge: failed to load baselines: %v", err)
	} else {
		e.baselines = baselines
	}

	rows, err := e.db.ListEvaluations(5000)
	if err != nil {
		log.Printf("verdikt/judge: failed to load evaluations: %v", err)
		return
	}
	// rows come newest-first; reverse into chronological order
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		e.history = append(e.history, Evaluation{
			ID:        fmt.Sprintf("eval-db-%d", r.ID),
			RequestID: r.RequestID,
			Score:     r.Score,
			Dimensions: Scores{
				Relevance:   r.Relevance,
				Accuracy:    r.Accuracy,
				Helpfulness: r.Helpfulness,
				Safety:      r.Safety,
				Tone:        r.Tone,
			},
			Verdict: r.Verdict,
			Issues:  r.Issues,
		})
	}
}

// Evaluate judges a single AI interaction.
func (e *Engine) Evaluate(ctx context.Context, interaction Interaction) (*Evaluation, error) {
	start := time.Now()

	domain := interaction.Domain
	if domain == "" {
		domain = e.domain
	}
	rb := e.rubricFor(domain)

	prompt := buildJudgePrompt(interaction, e.domain, rb)

	temp := 0.1
	maxTokens := 500
	resp, err := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("judge LLM failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from judge")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var eval Evaluation
	if err := json.Unmarshal([]byte(raw), &eval); err != nil {
		// Fallback
		eval = Evaluation{Score: 0.5, Verdict: "uncertain"}
	}

	eval.ID = fmt.Sprintf("eval-%d", time.Now().UnixNano())
	eval.RequestID = interaction.RequestID
	eval.Timestamp = time.Now()
	eval.LatencyMs = int(time.Since(start).Milliseconds())

	// Compute overall score from dimensions using rubric weights
	if eval.Score == 0 && eval.Dimensions != (Scores{}) {
		dims := map[string]float64{
			"relevance":   eval.Dimensions.Relevance,
			"accuracy":    eval.Dimensions.Accuracy,
			"helpfulness": eval.Dimensions.Helpfulness,
			"safety":      eval.Dimensions.Safety,
			"tone":        eval.Dimensions.Tone,
		}
		eval.Score = rb.WeightedScore(dims)
	}

	// Determine verdict
	switch {
	case eval.Score >= 0.85:
		eval.Verdict = "good"
	case eval.Score >= 0.65:
		eval.Verdict = "acceptable"
	case eval.Score >= 0.4:
		eval.Verdict = "poor"
	default:
		eval.Verdict = "bad"
	}

	e.mu.Lock()
	e.history = append(e.history, eval)
	if len(e.history) > 5000 {
		e.history = e.history[len(e.history)-5000:]
	}
	e.mu.Unlock()

	if e.db != nil {
		_ = e.db.SaveEvaluation(store.Evaluation{
			RequestID:   eval.RequestID,
			Domain:      interaction.Domain,
			Score:       eval.Score,
			Relevance:   eval.Dimensions.Relevance,
			Accuracy:    eval.Dimensions.Accuracy,
			Helpfulness: eval.Dimensions.Helpfulness,
			Safety:      eval.Dimensions.Safety,
			Tone:        eval.Dimensions.Tone,
			Verdict:     eval.Verdict,
			Issues:      eval.Issues,
			LatencyMs:   float64(eval.LatencyMs),
		})
	}

	return &eval, nil
}

// RecentEvaluations returns the last N evaluations.
func (e *Engine) RecentEvaluations(n int) []Evaluation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > len(e.history) {
		n = len(e.history)
	}
	out := make([]Evaluation, n)
	copy(out, e.history[len(e.history)-n:])
	return out
}

// Stats returns aggregate quality statistics.
type Stats struct {
	Total       int            `json:"total_evaluations"`
	AvgScore    float64        `json:"avg_score"`
	ByVerdict   map[string]int `json:"by_verdict"`
	RecentTrend float64        `json:"recent_trend"` // positive = improving
}

func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s := Stats{
		Total:     len(e.history),
		ByVerdict: map[string]int{},
	}

	if len(e.history) == 0 {
		return s
	}

	var sum float64
	for _, ev := range e.history {
		sum += ev.Score
		s.ByVerdict[ev.Verdict]++
	}
	s.AvgScore = sum / float64(len(e.history))

	// Trend: compare last 20% vs first 80%
	if len(e.history) >= 10 {
		split := len(e.history) * 8 / 10
		var earlySum, lateSum float64
		for _, ev := range e.history[:split] {
			earlySum += ev.Score
		}
		for _, ev := range e.history[split:] {
			lateSum += ev.Score
		}
		earlyAvg := earlySum / float64(split)
		lateAvg := lateSum / float64(len(e.history)-split)
		s.RecentTrend = lateAvg - earlyAvg
	}

	return s
}

func buildJudgePrompt(i Interaction, domain string, rb *rubric.Rubric) string {
	var sb strings.Builder

	sb.WriteString("You are an AI quality judge. Evaluate this AI response.\n\n")
	sb.WriteString("Respond with ONLY JSON:\n")
	sb.WriteString(`{"score":0.0-1.0,"dimensions":{"relevance":0.0-1.0,"accuracy":0.0-1.0,"helpfulness":0.0-1.0,"safety":0.0-1.0,"tone":0.0-1.0},"issues":["issue1","issue2"]}`)
	sb.WriteString("\n\n")

	if domain != "" {
		sb.WriteString(fmt.Sprintf("DOMAIN: %s\n\n", domain))
	}

	// Inject rubric weights so the LLM knows the scoring emphasis
	if rb != nil && len(rb.DimensionWeights) > 0 {
		sb.WriteString("SCORING WEIGHTS:\n")
		for dim, w := range rb.DimensionWeights {
			sb.WriteString(fmt.Sprintf("  %s: %.0f%%\n", dim, w*100))
		}
		sb.WriteString("\n")
	}

	// Inject custom criteria from rubric
	if rb != nil {
		if block := rb.CriteriaBlock(); block != "" {
			sb.WriteString(block)
			sb.WriteString("\n")
		}
	}

	// Inject few-shot examples from rubric
	if rb != nil && len(rb.Examples) > 0 {
		sb.WriteString("REFERENCE EXAMPLES:\n")
		for idx, ex := range rb.Examples {
			sb.WriteString(fmt.Sprintf("  Example %d — Prompt: %s | Response: %s | Expected score: %.2f",
				idx+1, truncate(ex.Prompt, 200), truncate(ex.Response, 200), ex.Score))
			if ex.Notes != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", ex.Notes))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("USER PROMPT:\n")
	sb.WriteString(truncate(i.Prompt, 2000))
	sb.WriteString("\n\nAI RESPONSE:\n")
	sb.WriteString(truncate(i.Response, 3000))

	if i.Context != "" {
		sb.WriteString("\n\nSYSTEM CONTEXT:\n")
		sb.WriteString(truncate(i.Context, 1000))
	}

	if i.UserReaction != "" {
		sb.WriteString(fmt.Sprintf("\n\nUSER REACTION: %s", i.UserReaction))
	}

	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
