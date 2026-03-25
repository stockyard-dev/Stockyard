// Package conversation manages multi-turn conversations between synthetic users and target apps.
package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/stampede/persona"
	"github.com/stockyard-dev/stockyard/internal/stampede/target"
)

// Result holds the complete outcome of a single conversation.
type Result struct {
	ID          string    `json:"id"`
	PersonaName string    `json:"persona_name"`
	Messages    []Message `json:"messages"`
	Status      string    `json:"status"` // completed, failed, timeout, error
	GoalResult  string    `json:"goal_result,omitempty"`
	GoalReason  string    `json:"goal_reason,omitempty"`
	TurnCount   int       `json:"turn_count"`
	TotalCost   float64   `json:"total_cost_cents"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Duration    time.Duration `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
}

// Message represents a single turn in a conversation.
type Message struct {
	Role      string        `json:"role"` // "user" (synthetic) or "assistant" (target)
	Content   string        `json:"content"`
	Latency   time.Duration `json:"latency_ms"`
	Tokens    int           `json:"tokens,omitempty"`
	CostCents float64       `json:"cost_cents,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// Config holds settings for a single conversation run.
type Config struct {
	MaxTurns int
	Verbose  bool
	Model    string
}

// Run executes a single conversation between a synthetic user and a target app.
func Run(ctx context.Context, id string, p persona.Persona, llm provider.Provider, tgt *target.Client, cfg Config) Result {
	result := Result{
		ID:          id,
		PersonaName: p.Name,
		StartedAt:   time.Now(),
		Status:      "active",
	}

	systemPrompt := BuildSystemPrompt(p)
	history := []provider.Message{
		{Role: "system", Content: systemPrompt},
	}

	// Generate opening message from synthetic user
	openingPrompt := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Begin the conversation. Send your first message to the support system as this character. Remember: you ARE the user, not an assistant."},
	}

	opening, openingCost, err := callLLM(ctx, llm, openingPrompt, cfg.Model)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("generating opening: %v", err)
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
		return result
	}

	// Add opening to message history
	result.Messages = append(result.Messages, Message{
		Role:      "user",
		Content:   opening,
		CostCents: openingCost,
		Timestamp: time.Now(),
	})
	history = append(history, provider.Message{Role: "assistant", Content: opening})
	result.TotalCost += openingCost

	if cfg.Verbose {
		fmt.Printf("  [%s] > %s\n", p.Name, truncate(opening, 120))
	}

	// Conversation loop
	for turn := 0; turn < cfg.MaxTurns; turn++ {
		if ctx.Err() != nil {
			result.Status = "timeout"
			break
		}

		// Send user message to target app
		targetStart := time.Now()
		targetResp, err := tgt.Send(ctx, opening)
		targetLatency := time.Since(targetStart)

		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("target error on turn %d: %v", turn+1, err)
			break
		}

		result.Messages = append(result.Messages, Message{
			Role:      "assistant",
			Content:   targetResp,
			Latency:   targetLatency,
			Timestamp: time.Now(),
		})

		if cfg.Verbose {
			fmt.Printf("  [target]%s < %s\n", strings.Repeat(" ", max(0, 19-len("target"))), truncate(targetResp, 120))
		}

		// Add target response to LLM history
		history = append(history, provider.Message{Role: "user", Content: targetResp})

		// Generate next synthetic user message
		nextMsg, nextCost, err := callLLM(ctx, llm, history, cfg.Model)
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("generating reply on turn %d: %v", turn+1, err)
			break
		}

		result.TotalCost += nextCost

		// Check for conversation end signal
		if done, goalResult, reason := parseDoneSignal(nextMsg); done {
			result.Status = "completed"
			result.GoalResult = goalResult
			result.GoalReason = reason
			// Strip the [DONE:...] from the displayed message
			cleanMsg := strings.TrimSpace(strings.Split(nextMsg, "[DONE:")[0])
			if cleanMsg != "" {
				result.Messages = append(result.Messages, Message{
					Role:      "user",
					Content:   cleanMsg,
					CostCents: nextCost,
					Timestamp: time.Now(),
				})
			}
			if cfg.Verbose {
				fmt.Printf("  [%s] [DONE:%s] %s\n", p.Name, goalResult, reason)
			}
			break
		}

		result.Messages = append(result.Messages, Message{
			Role:      "user",
			Content:   nextMsg,
			CostCents: nextCost,
			Timestamp: time.Now(),
		})
		history = append(history, provider.Message{Role: "assistant", Content: nextMsg})
		opening = nextMsg // Next iteration sends this to target

		if cfg.Verbose {
			fmt.Printf("  [%s] > %s\n", p.Name, truncate(nextMsg, 120))
		}

		result.TurnCount = turn + 1
	}

	if result.Status == "active" {
		result.Status = "timeout"
		result.GoalResult = "failed"
		result.GoalReason = "conversation reached maximum turns without completing"
	}

	result.TurnCount = len(result.Messages) / 2 // approximate turns
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	return result
}

// BuildSystemPrompt generates the LLM system prompt from a persona definition.
func BuildSystemPrompt(p persona.Persona) string {
	var sb strings.Builder

	sb.WriteString("You are role-playing as a user of a software application.\n")
	sb.WriteString("You are NOT an AI assistant. You ARE the user. Stay in character at all times.\n\n")

	sb.WriteString("YOUR CHARACTER:\n")
	sb.WriteString(p.Description)
	sb.WriteString("\n\n")

	sb.WriteString("YOUR GOAL:\n")
	sb.WriteString(p.Goal)
	sb.WriteString("\n\n")

	sb.WriteString("YOUR BEHAVIOR:\n")
	sb.WriteString(fmt.Sprintf("- Patience level: %s", p.Behavior.Patience))
	if p.Behavior.Patience == "low" {
		sb.WriteString(" (give up after 2-3 failed attempts)")
	}
	sb.WriteString("\n")

	if p.Behavior.TypoRate > 0 {
		sb.WriteString(fmt.Sprintf("- Make typos roughly %.0f%% of the time\n", p.Behavior.TypoRate*100))
	}

	sb.WriteString(fmt.Sprintf("- Reading comprehension: %s", p.Behavior.ReadingComprehension))
	if p.Behavior.ReadingComprehension == "partial" {
		sb.WriteString(" (sometimes miss details in responses)")
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("- Specificity: %s", p.Behavior.Specificity))
	if p.Behavior.Specificity == "low" {
		sb.WriteString(" (ask vague questions)")
	} else if p.Behavior.Specificity == "high" {
		sb.WriteString(" (ask precise, detailed questions)")
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("- Follow-up depth: %s\n", p.Behavior.FollowUpDepth))

	if p.Behavior.Escalation != "none" && p.Behavior.Escalation != "" {
		sb.WriteString(fmt.Sprintf("- Escalation: %s\n", p.Behavior.Escalation))
	}

	if len(p.Behavior.Techniques) > 0 {
		sb.WriteString(fmt.Sprintf("- Techniques to use: %s\n", strings.Join(p.Behavior.Techniques, ", ")))
	}

	sb.WriteString("\nRULES:\n")
	sb.WriteString("1. Act naturally. Real users don't write perfect prose.\n")
	sb.WriteString("2. If confused by a response, say so in character.\n")
	sb.WriteString("3. If frustrated, express it the way your character would.\n")
	sb.WriteString("4. When your goal is achieved OR you've given up, end your message with\n")
	sb.WriteString("   [DONE:completed] or [DONE:failed] followed by a brief reason.\n")
	sb.WriteString("5. Do NOT break character. Do NOT acknowledge you are an AI.\n")
	sb.WriteString("6. Keep messages short and natural. Real users don't write essays.\n")
	sb.WriteString("7. Respond ONLY with your message as the user. No narration or stage directions.\n")

	return sb.String()
}

// callLLM sends a request to the synthetic user's LLM and returns the response text and cost.
func callLLM(ctx context.Context, llm provider.Provider, messages []provider.Message, model string) (string, float64, error) {
	temp := 0.8
	maxTokens := 300
	resp, err := llm.Send(ctx, &provider.Request{
		Model:       model,
		Messages:    messages,
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return "", 0, err
	}

	if len(resp.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices in LLM response")
	}

	content := resp.Choices[0].Message.Content
	// Rough cost estimate (will be refined with actual pricing data)
	cost := float64(resp.Usage.TotalTokens) * 0.00015 // ~$0.15/1M tokens for mini models
	return content, cost, nil
}

// parseDoneSignal checks if a message contains the [DONE:result] signal.
func parseDoneSignal(msg string) (done bool, result string, reason string) {
	for _, tag := range []string{"[DONE:completed]", "[DONE:failed]", "[DONE:partial]"} {
		if idx := strings.Index(msg, tag); idx >= 0 {
			result = strings.TrimPrefix(strings.TrimSuffix(tag, "]"), "[DONE:")
			reason = strings.TrimSpace(msg[idx+len(tag):])
			if reason == "" {
				reason = "no reason provided"
			}
			return true, result, reason
		}
	}
	return false, "", ""
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
