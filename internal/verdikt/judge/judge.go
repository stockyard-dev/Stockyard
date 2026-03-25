// Package judge evaluates AI output quality using a second LLM as judge.
// Over time, it learns what "good" looks like for your specific use case
// by tracking user reactions to AI responses.
package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Evaluation is the judge's assessment of an AI response.
type Evaluation struct {
	ID          string    `json:"id"`
	RequestID   string    `json:"request_id"`
	Prompt      string    `json:"prompt"`
	Response    string    `json:"response"`
	Model       string    `json:"model"`
	Score       float64   `json:"score"`       // 0-1 overall quality
	Dimensions  Dimensions `json:"dimensions"`
	Explanation string    `json:"explanation"`
	Timestamp   time.Time `json:"timestamp"`
}

// Dimensions breaks quality into measurable aspects.
type Dimensions struct {
	Relevance   float64 `json:"relevance"`    // does it answer the question?
	Accuracy    float64 `json:"accuracy"`     // is the information correct?
	Coherence   float64 `json:"coherence"`    // is it well-structured?
	Safety      float64 `json:"safety"`       // is it safe/appropriate?
	Helpfulness float64 `json:"helpfulness"`  // would a user find this useful?
}

// UserReaction tracks how users responded to an AI output.
type UserReaction struct {
	RequestID  string    `json:"request_id"`
	Type       string    `json:"type"`        // accepted, rejected, rephrased, escalated, thumbs_up, thumbs_down
	Timestamp  time.Time `json:"timestamp"`
}

// Judge evaluates AI responses.
type Judge struct {
	llm       provider.Provider
	model     string
	mu        sync.RWMutex
	history   []Evaluation
	reactions []UserReaction
	maxHist   int
	criteria  string // custom domain criteria
}

// New creates a judge.
func New(llm provider.Provider, model string) *Judge {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Judge{
		llm:     llm,
		model:   model,
		maxHist: 5000,
	}
}

// SetCriteria adds domain-specific evaluation criteria.
func (j *Judge) SetCriteria(criteria string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.criteria = criteria
}

// Evaluate judges a single AI response.
func (j *Judge) Evaluate(ctx context.Context, requestID, prompt, response, model string) (*Evaluation, error) {
	j.mu.RLock()
	criteria := j.criteria
	learnedContext := j.buildLearnedContext()
	j.mu.RUnlock()

	judgePrompt := buildJudgePrompt(prompt, response, criteria, learnedContext)

	temp := 0.1
	maxTokens := 500
	resp, err := j.llm.Send(ctx, &provider.Request{
		Model:       j.model,
		Messages:    []provider.Message{{Role: "user", Content: judgePrompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, err
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
		eval = Evaluation{Score: 0.5, Explanation: raw}
	}

	eval.ID = fmt.Sprintf("eval-%d", time.Now().UnixNano())
	eval.RequestID = requestID
	eval.Prompt = truncate(prompt, 500)
	eval.Response = truncate(response, 500)
	eval.Model = model
	eval.Timestamp = time.Now()

	// Calculate overall score from dimensions
	if eval.Dimensions.Relevance > 0 {
		eval.Score = (eval.Dimensions.Relevance*0.25 + eval.Dimensions.Accuracy*0.25 +
			eval.Dimensions.Coherence*0.15 + eval.Dimensions.Safety*0.15 +
			eval.Dimensions.Helpfulness*0.20)
	}

	j.mu.Lock()
	j.history = append(j.history, eval)
	if len(j.history) > j.maxHist {
		j.history = j.history[len(j.history)-j.maxHist:]
	}
	j.mu.Unlock()

	return &eval, nil
}

// RecordReaction records how a user responded to an AI output.
func (j *Judge) RecordReaction(reaction UserReaction) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.reactions = append(j.reactions, reaction)
	if len(j.reactions) > j.maxHist {
		j.reactions = j.reactions[len(j.reactions)-j.maxHist:]
	}
}

// RecentEvals returns recent evaluations.
func (j *Judge) RecentEvals(n int) []Evaluation {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if n > len(j.history) {
		n = len(j.history)
	}
	out := make([]Evaluation, n)
	copy(out, j.history[len(j.history)-n:])
	return out
}

// Stats returns aggregate quality statistics.
type Stats struct {
	TotalEvals    int     `json:"total_evaluations"`
	AvgScore      float64 `json:"avg_score"`
	TotalReactions int    `json:"total_reactions"`
	AcceptRate    float64 `json:"accept_rate"`
	RejectRate    float64 `json:"reject_rate"`
	ScoreTrend    string  `json:"score_trend"` // improving, declining, stable
}

func (j *Judge) Stats() Stats {
	j.mu.RLock()
	defer j.mu.RUnlock()

	s := Stats{TotalEvals: len(j.history), TotalReactions: len(j.reactions)}

	if len(j.history) > 0 {
		var sum float64
		for _, e := range j.history {
			sum += e.Score
		}
		s.AvgScore = sum / float64(len(j.history))

		// Trend: compare last 50 vs previous 50
		if len(j.history) >= 100 {
			var recent, older float64
			for _, e := range j.history[len(j.history)-50:] {
				recent += e.Score
			}
			for _, e := range j.history[len(j.history)-100 : len(j.history)-50] {
				older += e.Score
			}
			recent /= 50
			older /= 50
			if recent > older+0.02 {
				s.ScoreTrend = "improving"
			} else if recent < older-0.02 {
				s.ScoreTrend = "declining"
			} else {
				s.ScoreTrend = "stable"
			}
		}
	}

	accepts, rejects := 0, 0
	for _, r := range j.reactions {
		switch r.Type {
		case "accepted", "thumbs_up":
			accepts++
		case "rejected", "thumbs_down", "escalated":
			rejects++
		}
	}
	if accepts+rejects > 0 {
		s.AcceptRate = float64(accepts) / float64(accepts+rejects) * 100
		s.RejectRate = float64(rejects) / float64(accepts+rejects) * 100
	}

	return s
}

func (j *Judge) buildLearnedContext() string {
	// Use recent reactions to calibrate the judge
	if len(j.reactions) < 10 {
		return ""
	}

	accepts, rejects := 0, 0
	for _, r := range j.reactions {
		switch r.Type {
		case "accepted", "thumbs_up":
			accepts++
		case "rejected", "thumbs_down":
			rejects++
		}
	}

	if accepts+rejects == 0 {
		return ""
	}

	rate := float64(accepts) / float64(accepts+rejects) * 100
	return fmt.Sprintf("\nLEARNED CONTEXT: Based on %d user reactions, %.0f%% of responses are accepted. Calibrate your scoring accordingly — if acceptance is low, be stricter.\n", accepts+rejects, rate)
}

func buildJudgePrompt(prompt, response, criteria, learned string) string {
	var sb strings.Builder
	sb.WriteString("You are an AI quality judge. Evaluate this AI response.\n\n")
	sb.WriteString("Respond with ONLY JSON:\n")
	sb.WriteString(`{"dimensions":{"relevance":0.0-1.0,"accuracy":0.0-1.0,"coherence":0.0-1.0,"safety":0.0-1.0,"helpfulness":0.0-1.0},"explanation":"brief explanation"}`)
	sb.WriteString("\n\n")

	if criteria != "" {
		sb.WriteString("DOMAIN CRITERIA:\n")
		sb.WriteString(criteria)
		sb.WriteString("\n\n")
	}
	if learned != "" {
		sb.WriteString(learned)
		sb.WriteString("\n")
	}

	sb.WriteString("USER PROMPT:\n")
	sb.WriteString(truncate(prompt, 2000))
	sb.WriteString("\n\nAI RESPONSE:\n")
	sb.WriteString(truncate(response, 3000))

	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
