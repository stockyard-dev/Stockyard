// Package tree defines decisions as first-class objects and evaluates them at runtime.
// Every piece of software is a tree of decisions. Grain makes them explicit, swappable,
// auditable, and A/B testable without deployments.
package tree

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/grain/store"
	"gopkg.in/yaml.v3"
)

// Decision is the atomic unit of Grain — a named branch point in your application.
type Decision struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Default     string            `yaml:"default" json:"default"`               // default outcome
	Variants    []Variant         `yaml:"variants,omitempty" json:"variants,omitempty"` // A/B test variants
	Rules       []Rule            `yaml:"rules,omitempty" json:"rules,omitempty"`       // conditional rules
	Tags        map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Locked      bool              `yaml:"locked,omitempty" json:"locked,omitempty"`     // prevent runtime changes
}

// Variant is an alternative outcome for A/B testing.
type Variant struct {
	Name   string  `yaml:"name" json:"name"`
	Value  string  `yaml:"value" json:"value"`
	Weight float64 `yaml:"weight" json:"weight"` // 0-100, percentage of traffic
}

// Rule is a conditional override — if the condition matches, use this outcome.
type Rule struct {
	Condition string `yaml:"condition" json:"condition"` // e.g. "user.plan == pro"
	Value     string `yaml:"value" json:"value"`
	Priority  int    `yaml:"priority,omitempty" json:"priority,omitempty"` // higher = evaluated first
}

// Outcome is the result of evaluating a decision.
type Outcome struct {
	DecisionID string    `json:"decision_id"`
	Value      string    `json:"value"`
	Reason     string    `json:"reason"` // why this value was chosen
	Variant    string    `json:"variant,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// EvalContext provides context for decision evaluation (user info, request data, etc.)
type EvalContext struct {
	UserID    string            `json:"user_id,omitempty"`
	Plan      string            `json:"plan,omitempty"`
	Country   string            `json:"country,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Custom    map[string]string `json:"custom,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

// Registry holds all decisions and evaluates them at runtime.
type Registry struct {
	mu        sync.RWMutex
	decisions map[string]*Decision
	overrides map[string]string // decision ID -> forced value
	rng       *rand.Rand
	db        *store.DB
}

// NewRegistry creates a decision registry.
func NewRegistry(db *store.DB) *Registry {
	r := &Registry{
		decisions: map[string]*Decision{},
		overrides: map[string]string{},
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		db:        db,
	}
	if db != nil {
		r.loadFromDB()
	}
	return r
}

// loadFromDB restores decisions and overrides from SQLite on startup.
func (r *Registry) loadFromDB() {
	decisions, err := r.db.ListDecisions()
	if err != nil {
		log.Printf("grain: loading decisions from db: %v", err)
		return
	}
	for id, sd := range decisions {
		d := storeToDecision(sd)
		r.decisions[id] = &d
	}

	overrides, err := r.db.GetOverrides()
	if err != nil {
		log.Printf("grain: loading overrides from db: %v", err)
		return
	}
	for id, val := range overrides {
		r.overrides[id] = val
	}
}

func decisionToStore(d Decision) store.Decision {
	var variants []store.Variant
	for _, v := range d.Variants {
		variants = append(variants, store.Variant{Name: v.Name, Value: v.Value, Weight: v.Weight})
	}
	var rules []store.Rule
	for _, r := range d.Rules {
		rules = append(rules, store.Rule{Condition: r.Condition, Value: r.Value, Priority: r.Priority})
	}
	return store.Decision{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Default:     d.Default,
		Variants:    variants,
		Rules:       rules,
		Tags:        d.Tags,
		Locked:      d.Locked,
	}
}

func storeToDecision(sd *store.Decision) Decision {
	var variants []Variant
	for _, v := range sd.Variants {
		variants = append(variants, Variant{Name: v.Name, Value: v.Value, Weight: v.Weight})
	}
	var rules []Rule
	for _, r := range sd.Rules {
		rules = append(rules, Rule{Condition: r.Condition, Value: r.Value, Priority: r.Priority})
	}
	return Decision{
		ID:          sd.ID,
		Name:        sd.Name,
		Description: sd.Description,
		Default:     sd.Default,
		Variants:    variants,
		Rules:       rules,
		Tags:        sd.Tags,
		Locked:      sd.Locked,
	}
}

// Register adds a decision to the registry.
func (r *Registry) Register(d Decision) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.decisions[d.ID] = &d
	if r.db != nil {
		if err := r.db.SaveDecision(decisionToStore(d)); err != nil {
			log.Printf("grain: saving decision %s: %v", d.ID, err)
		}
	}
}

// Evaluate resolves a decision to an outcome.
func (r *Registry) Evaluate(ctx context.Context, decisionID string, ec *EvalContext) Outcome {
	r.mu.RLock()
	d, ok := r.decisions[decisionID]
	override, hasOverride := r.overrides[decisionID]
	r.mu.RUnlock()

	if !ok {
		return Outcome{DecisionID: decisionID, Value: "", Reason: "decision not found", Timestamp: time.Now()}
	}

	// Check override first
	if hasOverride {
		return Outcome{DecisionID: decisionID, Value: override, Reason: "manual override", Timestamp: time.Now()}
	}

	// Check rules (highest priority first)
	if len(d.Rules) > 0 && ec != nil {
		for _, rule := range sortedRules(d.Rules) {
			if matchesCondition(rule.Condition, ec) {
				return Outcome{DecisionID: decisionID, Value: rule.Value, Reason: "rule: " + rule.Condition, Timestamp: time.Now()}
			}
		}
	}

	// Check A/B variants
	if len(d.Variants) > 0 {
		variant := r.selectVariant(d.Variants)
		return Outcome{DecisionID: decisionID, Value: variant.Value, Variant: variant.Name, Reason: "variant: " + variant.Name, Timestamp: time.Now()}
	}

	// Default
	return Outcome{DecisionID: decisionID, Value: d.Default, Reason: "default", Timestamp: time.Now()}
}

// Override forces a decision to a specific value (hot-swap without deployment).
func (r *Registry) Override(decisionID, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.decisions[decisionID]; ok && d.Locked {
		return // locked decisions can't be overridden
	}
	r.overrides[decisionID] = value
	if r.db != nil {
		if err := r.db.SetOverride(decisionID, value); err != nil {
			log.Printf("grain: saving override %s: %v", decisionID, err)
		}
	}
}

// ClearOverride removes an override, returning to normal evaluation.
func (r *Registry) ClearOverride(decisionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.overrides, decisionID)
	if r.db != nil {
		if err := r.db.DeleteOverride(decisionID); err != nil {
			log.Printf("grain: deleting override %s: %v", decisionID, err)
		}
	}
}

// ListDecisions returns all registered decisions.
func (r *Registry) ListDecisions() []Decision {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Decision
	for _, d := range r.decisions {
		out = append(out, *d)
	}
	return out
}

// GetDecision returns a single decision.
func (r *Registry) GetDecision(id string) *Decision {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if d, ok := r.decisions[id]; ok {
		cp := *d
		return &cp
	}
	return nil
}

// Count returns the number of registered decisions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.decisions)
}

// LoadFromFile loads decisions from a YAML file.
func (r *Registry) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config struct {
		Decisions []Decision `yaml:"decisions"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	for _, d := range config.Decisions {
		r.Register(d)
	}
	return nil
}

// ExportJSON returns all decisions as JSON.
func (r *Registry) ExportJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return json.MarshalIndent(r.decisions, "", "  ")
}

// selectVariant picks a variant based on traffic weights.
func (r *Registry) selectVariant(variants []Variant) Variant {
	roll := r.rng.Float64() * 100
	cumulative := 0.0
	for _, v := range variants {
		cumulative += v.Weight
		if roll < cumulative {
			return v
		}
	}
	return variants[len(variants)-1]
}

// matchesCondition evaluates a simple condition against context.
func matchesCondition(condition string, ec *EvalContext) bool {
	// Simple matching: "field == value" or "field != value"
	parts := splitCondition(condition)
	if len(parts) != 3 {
		return false
	}
	field, op, expected := parts[0], parts[1], parts[2]

	actual := resolveField(field, ec)

	switch op {
	case "==":
		return actual == expected
	case "!=":
		return actual != expected
	case "contains":
		return contains(actual, expected)
	default:
		return false
	}
}

func resolveField(field string, ec *EvalContext) string {
	switch field {
	case "user.id", "user_id":
		return ec.UserID
	case "user.plan", "plan":
		return ec.Plan
	case "user.country", "country":
		return ec.Country
	default:
		if ec.Custom != nil {
			return ec.Custom[field]
		}
		return ""
	}
}

func splitCondition(s string) []string {
	for _, op := range []string{" == ", " != ", " contains "} {
		if idx := indexOf(s, op); idx >= 0 {
			return []string{s[:idx], trimOp(op), s[idx+len(op):]}
		}
	}
	return nil
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimOp(s string) string {
	result := ""
	for _, c := range s {
		if c != ' ' {
			result += string(c)
		}
	}
	return result
}

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

func sortedRules(rules []Rule) []Rule {
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}
