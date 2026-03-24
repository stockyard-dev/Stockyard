// Package runner orchestrates concurrent synthetic user conversations.
package runner

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/stampede/analysis"
	"github.com/stockyard-dev/stockyard/internal/stampede/conversation"
	"github.com/stockyard-dev/stockyard/internal/stampede/persona"
	"github.com/stockyard-dev/stockyard/internal/stampede/store"
	"github.com/stockyard-dev/stockyard/internal/stampede/target"
)

// Config holds settings for a simulation run.
type Config struct {
	Personas      []persona.Persona
	Distribution  []float64 // weights corresponding to Personas (sum to 1.0)
	UserCount     int
	Duration      time.Duration
	RampUp        time.Duration
	MaxTurns      int
	RateLimit     int // max requests/sec, 0 = unlimited
	Verbose       bool
	ProviderName  string
	ProviderKey   string
	ProviderModel string
}

// Run executes a full simulation and returns aggregated results.
func Run(ctx context.Context, cfg Config, tgt *target.Client, db *store.DB) (*store.SimulationResult, error) {
	simID := uuid.New().String()[:8]

	// Create the LLM provider for synthetic users
	llm, err := makeProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating LLM provider: %w", err)
	}

	// Save simulation to DB
	configJSON := fmt.Sprintf(`{"users":%d,"duration":"%s","ramp_up":"%s","max_turns":%d}`,
		cfg.UserCount, cfg.Duration, cfg.RampUp, cfg.MaxTurns)
	if err := db.CreateSimulation(simID, tgt.URL(), cfg.UserCount, configJSON); err != nil {
		return nil, fmt.Errorf("saving simulation: %w", err)
	}

	// Results collection
	var (
		results   []conversation.Result
		resultsMu sync.Mutex
		wg        sync.WaitGroup
		active    int64
		completed int64
		totalCost float64
	)

	// Rate limiter
	var rateLimiter <-chan time.Time
	if cfg.RateLimit > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(cfg.RateLimit))
		defer ticker.Stop()
		rateLimiter = ticker.C
	}

	// Calculate ramp-up schedule
	rampInterval := time.Duration(0)
	if cfg.RampUp > 0 && cfg.UserCount > 1 {
		rampInterval = cfg.RampUp / time.Duration(cfg.UserCount)
	}

	// Deadline context
	deadline := time.Now().Add(cfg.Duration)
	simCtx, simCancel := context.WithDeadline(ctx, deadline)
	defer simCancel()

	// Progress display
	if !cfg.Verbose {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-simCtx.Done():
					return
				case <-ticker.C:
					a := atomic.LoadInt64(&active)
					c := atomic.LoadInt64(&completed)
					fmt.Printf("\r  Active: %d  |  Completed: %d/%d  |  Cost: $%.4f",
						a, c, cfg.UserCount, totalCost)
				}
			}
		}()
	}

	// Launch synthetic users
	for i := 0; i < cfg.UserCount; i++ {
		if simCtx.Err() != nil {
			break
		}

		// Ramp-up delay
		if rampInterval > 0 && i > 0 {
			select {
			case <-time.After(rampInterval):
			case <-simCtx.Done():
				break
			}
		}

		// Rate limit
		if rateLimiter != nil {
			select {
			case <-rateLimiter:
			case <-simCtx.Done():
				break
			}
		}

		// Select persona based on distribution
		p := selectPersona(cfg.Personas, cfg.Distribution)
		convID := fmt.Sprintf("%s-%03d", simID, i+1)

		wg.Add(1)
		atomic.AddInt64(&active, 1)

		go func(p persona.Persona, id string) {
			defer wg.Done()
			defer atomic.AddInt64(&active, -1)

			result := conversation.Run(simCtx, id, p, llm, tgt, conversation.Config{
				MaxTurns: cfg.MaxTurns,
				Verbose:  cfg.Verbose,
				Model:    cfg.ProviderModel,
			})

			// Save to DB
			msgsJSON := store.MarshalMessages(result.Messages)
			db.SaveConversation(simID, result.ID, result.PersonaName, result.Status,
				result.GoalResult, result.GoalReason, result.TurnCount,
				result.TotalCost, result.Duration.Milliseconds(), msgsJSON, result.Error)

			resultsMu.Lock()
			results = append(results, result)
			totalCost += result.TotalCost
			resultsMu.Unlock()

			atomic.AddInt64(&completed, 1)

			if cfg.Verbose {
				fmt.Printf("  [%s] Conversation %s: %s (%s) — %d turns, $%.4f\n",
					result.PersonaName, result.ID, result.GoalResult, result.GoalReason,
					result.TurnCount, result.TotalCost)
			}
		}(p, convID)
	}

	// Wait for all conversations to complete
	wg.Wait()
	if !cfg.Verbose {
		fmt.Println() // Clear progress line
	}

	// Run analysis (goal grading + safety scanning)
	fmt.Println()
	analysisCtx, analysisCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer analysisCancel()

	analysisResult, err := analysis.Analyze(analysisCtx, results, llm, analysis.Config{
		GraderModel: cfg.ProviderModel,
		Concurrency: 5,
	})
	if err != nil {
		fmt.Printf("  Warning: analysis failed: %v (results still saved)\n", err)
	} else {
		// Save grades to DB
		for _, g := range analysisResult.Grades {
			db.SaveGrade(simID, g.ConversationID, g.Result, g.Confidence, g.Reason, g.FailurePoint, g.Suggestion)
		}
		// Save safety findings to DB
		for _, f := range analysisResult.SafetyFindings {
			db.SaveSafetyFinding(simID, f.ConversationID, f.Type, f.Severity, f.Description, f.TurnNumber, f.Evidence)
		}
		fmt.Printf("  Analysis complete: %d graded, %d safety findings\n",
			len(analysisResult.Grades), len(analysisResult.SafetyFindings))
	}

	// Complete simulation in DB
	db.CompleteSimulation(simID, len(results), totalCost)

	// Load full result from DB for reporting
	return db.LoadSimulationResult(simID)
}

// selectPersona picks a persona based on weighted distribution.
func selectPersona(personas []persona.Persona, dist []float64) persona.Persona {
	r := rand.Float64()
	cumulative := 0.0
	for i, w := range dist {
		cumulative += w
		if r <= cumulative {
			return personas[i]
		}
	}
	return personas[len(personas)-1]
}

// EqualDistribution returns equal weights for all personas.
func EqualDistribution(personas []persona.Persona) []float64 {
	w := 1.0 / float64(len(personas))
	dist := make([]float64, len(personas))
	for i := range dist {
		dist[i] = w
	}
	return dist
}

// ParseDistribution parses a distribution string like "confused:60%,power:25%,adversarial:15%"
func ParseDistribution(s string, personas []persona.Persona) ([]float64, error) {
	dist := make([]float64, len(personas))
	nameIdx := map[string]int{}
	for i, p := range personas {
		nameIdx[p.Name] = i
	}

	parts := strings.Split(s, ",")
	totalPct := 0.0
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid distribution part: %q (expected name:pct%%)", part)
		}
		name := strings.TrimSpace(kv[0])
		pctStr := strings.TrimSpace(strings.TrimSuffix(kv[1], "%"))
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid percentage for %s: %q", name, kv[1])
		}

		// Find matching persona (partial match)
		found := false
		for pName, idx := range nameIdx {
			if strings.Contains(pName, name) || strings.Contains(name, pName) {
				dist[idx] = pct / 100.0
				totalPct += pct
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("persona %q not found in loaded personas", name)
		}
	}

	// Normalize if doesn't sum to 100
	if totalPct > 0 && totalPct != 100 {
		for i := range dist {
			dist[i] /= (totalPct / 100.0)
		}
	}

	return dist, nil
}

// makeProvider creates an LLM provider for synthetic user thinking.
func makeProvider(cfg Config) (provider.Provider, error) {
	pcfg := provider.ProviderConfig{
		APIKey:  cfg.ProviderKey,
		Timeout: 60 * time.Second,
	}

	switch strings.ToLower(cfg.ProviderName) {
	case "openai":
		return provider.NewOpenAI(pcfg), nil
	case "anthropic":
		return provider.NewAnthropic(pcfg), nil
	default:
		return provider.NewOpenAI(pcfg), nil
	}
}
