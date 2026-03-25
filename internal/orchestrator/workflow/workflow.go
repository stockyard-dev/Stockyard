// Package workflow defines predefined automation chains that connect products.
// When an error is detected, the chain fires: Fault → Replay → Séance → Iron → Stampede.
// When code is generated, the chain fires: Iron → Doubt → Hollow → Stampede → Verdikt.
package workflow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/hub"
)

// Chain is a sequence of steps that execute in order when triggered.
type Chain struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Trigger     bus.EventType `json:"trigger"`
	Steps       []Step    `json:"steps"`
	Enabled     bool      `json:"enabled"`
	Runs        int       `json:"runs"`
	LastRun     time.Time `json:"last_run"`
	AvgMs       int       `json:"avg_ms"`
}

// Step is a single action in a chain.
type Step struct {
	ProductID string         `json:"product_id"` // which product handles this
	Action    string         `json:"action"`     // what to do
	EmitEvent bus.EventType  `json:"emit_event"` // event to emit on success
	Timeout   time.Duration  `json:"timeout"`
	Optional  bool           `json:"optional"`   // chain continues if this fails
}

// StepResult holds what happened when a step ran.
type StepResult struct {
	ProductID string        `json:"product_id"`
	Action    string        `json:"action"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Output    map[string]any `json:"output,omitempty"`
}

// RunResult holds the full chain execution result.
type RunResult struct {
	ChainID   string       `json:"chain_id"`
	TraceID   string       `json:"trace_id"`
	Success   bool         `json:"success"`
	Steps     []StepResult `json:"steps"`
	TotalMs   int          `json:"total_ms"`
	Timestamp time.Time    `json:"timestamp"`
	Trigger   bus.Event    `json:"trigger"`
}

// Engine manages and executes workflow chains.
type Engine struct {
	mu       sync.RWMutex
	chains   map[string]*Chain
	eventBus *bus.Bus
	hub      *hub.Hub
	history  []RunResult
	maxHist  int
}

// New creates a workflow engine.
func New(eventBus *bus.Bus, h *hub.Hub) *Engine {
	e := &Engine{
		chains:   map[string]*Chain{},
		eventBus: eventBus,
		hub:      h,
		maxHist:  500,
	}
	return e
}

// Register adds a chain and subscribes it to the event bus.
func (e *Engine) Register(chain Chain) {
	e.mu.Lock()
	chain.Enabled = true
	e.chains[chain.ID] = &chain
	e.mu.Unlock()

	// Subscribe to trigger event
	e.eventBus.Subscribe(chain.Trigger, "", func(event bus.Event) {
		e.mu.RLock()
		c, ok := e.chains[chain.ID]
		if !ok || !c.Enabled {
			e.mu.RUnlock()
			return
		}
		e.mu.RUnlock()

		result := e.Execute(context.Background(), chain.ID, event)
		if !result.Success {
			log.Printf("[workflow] chain %s failed on trigger %s: %v", chain.ID, event.Type, result)
		}
	})
}

// Execute runs a chain manually or from an event trigger.
func (e *Engine) Execute(ctx context.Context, chainID string, trigger bus.Event) RunResult {
	e.mu.RLock()
	chain, ok := e.chains[chainID]
	if !ok {
		e.mu.RUnlock()
		return RunResult{ChainID: chainID, Success: false, Steps: []StepResult{{Error: "chain not found"}}}
	}
	steps := make([]Step, len(chain.Steps))
	copy(steps, chain.Steps)
	e.mu.RUnlock()

	start := time.Now()
	traceID := trigger.TraceID
	if traceID == "" {
		traceID = fmt.Sprintf("trace-%d", time.Now().UnixNano()%100000000)
	}

	result := RunResult{
		ChainID:   chainID,
		TraceID:   traceID,
		Success:   true,
		Timestamp: time.Now(),
		Trigger:   trigger,
	}

	// Pass data forward through steps
	stepData := trigger.Data
	if stepData == nil {
		stepData = map[string]any{}
	}

	for _, step := range steps {
		stepStart := time.Now()

		// Check if the product is available
		product := e.hub.Get(step.ProductID)
		if product == nil {
			sr := StepResult{
				ProductID: step.ProductID,
				Action:    step.Action,
				Success:   false,
				Error:     "product not registered",
				Duration:  time.Since(stepStart),
			}
			result.Steps = append(result.Steps, sr)
			if !step.Optional {
				result.Success = false
				break
			}
			continue
		}

		// Execute the step (emit an event for the target product)
		stepCtx := ctx
		if step.Timeout > 0 {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
			defer cancel()
		}

		_ = stepCtx // used by future implementations that call product APIs directly

		// For now, emit an event that the product should pick up
		if step.EmitEvent != "" {
			e.eventBus.Emit(bus.Event{
				Type:    step.EmitEvent,
				Source:  "orchestrator",
				TraceID: traceID,
				Data:    stepData,
			})
		}

		sr := StepResult{
			ProductID: step.ProductID,
			Action:    step.Action,
			Success:   true,
			Duration:  time.Since(stepStart),
			Output:    stepData,
		}
		result.Steps = append(result.Steps, sr)
	}

	result.TotalMs = int(time.Since(start).Milliseconds())

	// Record in history
	e.mu.Lock()
	chain.Runs++
	chain.LastRun = time.Now()
	if chain.AvgMs == 0 {
		chain.AvgMs = result.TotalMs
	} else {
		chain.AvgMs = (chain.AvgMs + result.TotalMs) / 2
	}
	e.history = append(e.history, result)
	if len(e.history) > e.maxHist {
		e.history = e.history[len(e.history)-e.maxHist:]
	}
	e.mu.Unlock()

	return result
}

// ListChains returns all registered chains.
func (e *Engine) ListChains() []Chain {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []Chain
	for _, c := range e.chains {
		out = append(out, *c)
	}
	return out
}

// RecentRuns returns the last N execution results.
func (e *Engine) RecentRuns(n int) []RunResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > len(e.history) {
		n = len(e.history)
	}
	out := make([]RunResult, n)
	copy(out, e.history[len(e.history)-n:])
	return out
}

// EnableChain toggles a chain on/off.
func (e *Engine) EnableChain(id string, enabled bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.chains[id]; ok {
		c.Enabled = enabled
		return true
	}
	return false
}

// RegisterDefaults loads the built-in automation chains.
func RegisterDefaults(e *Engine) {
	// ========= Error → Diagnose → Patch → Test → Deploy =========
	e.Register(Chain{
		ID:          "self-heal",
		Name:        "Self-Healing Pipeline",
		Description: "When an error is detected: capture state, diagnose, generate patch, test, deploy",
		Trigger:     bus.EventErrorDetected,
		Steps: []Step{
			{ProductID: "replay", Action: "capture_state", EmitEvent: bus.EventStateRecorded, Timeout: 5 * time.Second},
			{ProductID: "fault", Action: "diagnose", EmitEvent: bus.EventErrorDiagnosed, Timeout: 30 * time.Second},
			{ProductID: "fault", Action: "generate_patch", EmitEvent: bus.EventPatchGenerated, Timeout: 30 * time.Second},
			{ProductID: "stampede", Action: "test_patch", EmitEvent: bus.EventTestsPassed, Timeout: 60 * time.Second, Optional: true},
			{ProductID: "echo", Action: "record_change", EmitEvent: bus.EventAnnotationAdded, Timeout: 5 * time.Second, Optional: true},
		},
	})

	// ========= Spec → Code → Test → Score → Deploy =========
	e.Register(Chain{
		ID:          "spec-to-prod",
		Name:        "Spec to Production",
		Description: "From spec to deployed, tested, scored code",
		Trigger:     bus.EventSpecCreated,
		Steps: []Step{
			{ProductID: "iron", Action: "generate_code", EmitEvent: bus.EventCodeGenerated, Timeout: 120 * time.Second},
			{ProductID: "doubt", Action: "score_confidence", EmitEvent: bus.EventQualityScored, Timeout: 10 * time.Second},
			{ProductID: "hollow", Action: "find_gaps", EmitEvent: bus.EventGapFound, Timeout: 15 * time.Second, Optional: true},
			{ProductID: "stampede", Action: "run_synthetic_tests", EmitEvent: bus.EventTestsPassed, Timeout: 120 * time.Second},
			{ProductID: "trailhead", Action: "index_new_code", EmitEvent: bus.EventIndexComplete, Timeout: 30 * time.Second, Optional: true},
		},
	})

	// ========= Quality Degradation → Investigate =========
	e.Register(Chain{
		ID:          "quality-alert",
		Name:        "Quality Alert Pipeline",
		Description: "When AI output quality drops: investigate drift, check confidence, alert",
		Trigger:     bus.EventQualityDegraded,
		Steps: []Step{
			{ProductID: "doubt", Action: "check_confidence", EmitEvent: bus.EventConfidenceLow, Timeout: 10 * time.Second},
			{ProductID: "seance", Action: "investigate", EmitEvent: bus.EventQueryAnswered, Timeout: 30 * time.Second},
			{ProductID: "prism", Action: "check_user_impact", EmitEvent: bus.EventUserConfused, Timeout: 10 * time.Second, Optional: true},
		},
	})

	// ========= Pressure → Hibernate → Scale =========
	e.Register(Chain{
		ID:          "pressure-response",
		Name:        "Pressure Response",
		Description: "When system pressure rises: hibernate luxuries, then scale if needed",
		Trigger:     bus.EventPressureHigh,
		Steps: []Step{
			{ProductID: "tide", Action: "hibernate_non_critical", EmitEvent: bus.EventFeatureHibernated, Timeout: 5 * time.Second},
			{ProductID: "spine", Action: "evaluate_scaling", EmitEvent: bus.EventScaleUp, Timeout: 10 * time.Second, Optional: true},
			{ProductID: "bid", Action: "switch_to_cheapest", EmitEvent: bus.EventAuctionComplete, Timeout: 5 * time.Second, Optional: true},
		},
	})

	// ========= New Code → Knowledge Update =========
	e.Register(Chain{
		ID:          "knowledge-sync",
		Name:        "Knowledge Sync",
		Description: "When new code lands: update knowledge base, check for dead code, record history",
		Trigger:     bus.EventCodeGenerated,
		Steps: []Step{
			{ProductID: "trailhead", Action: "index", EmitEvent: bus.EventIndexComplete, Timeout: 60 * time.Second},
			{ProductID: "echo", Action: "record_version", EmitEvent: bus.EventAnnotationAdded, Timeout: 10 * time.Second},
			{ProductID: "fossil", Action: "check_dead_code", EmitEvent: bus.EventDeadCodeFound, Timeout: 15 * time.Second, Optional: true},
			{ProductID: "doubt", Action: "score_new_code", EmitEvent: bus.EventConfidenceLow, Timeout: 10 * time.Second, Optional: true},
		},
	})
}
