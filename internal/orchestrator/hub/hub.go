// Package hub is the central registry of all Stockyard products.
// It knows what's running, their health, capabilities, and how to reach them.
package hub

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Product represents a registered Stockyard product.
type Product struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Category     string            `json:"category"`     // lifecycle, runtime, quality, infra, insight
	Status       Status            `json:"status"`
	HealthURL    string            `json:"health_url,omitempty"`
	APIURL       string            `json:"api_url,omitempty"`
	UIURL        string            `json:"ui_url,omitempty"`
	Capabilities []string          `json:"capabilities"` // what events it can handle
	Emits        []string          `json:"emits"`        // what events it produces
	LastCheck    time.Time         `json:"last_check"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Status represents product health.
type Status string

const (
	StatusHealthy    Status = "healthy"
	StatusDegraded   Status = "degraded"
	StatusDown       Status = "down"
	StatusUnknown    Status = "unknown"
	StatusEmbedded   Status = "embedded" // running in-process
)

// Hub manages all registered products.
type Hub struct {
	mu       sync.RWMutex
	products map[string]*Product
	client   *http.Client
	stopCh   chan struct{}
}

// New creates a product hub.
func New() *Hub {
	return &Hub{
		products: map[string]*Product{},
		client:   &http.Client{Timeout: 5 * time.Second},
		stopCh:   make(chan struct{}),
	}
}

// Register adds a product to the hub.
func (h *Hub) Register(p Product) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p.Status == "" {
		p.Status = StatusUnknown
	}
	h.products[p.ID] = &p
}

// Get returns a product by ID.
func (h *Hub) Get(id string) *Product {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if p, ok := h.products[id]; ok {
		cp := *p
		return &cp
	}
	return nil
}

// All returns all registered products.
func (h *Hub) All() []Product {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []Product
	for _, p := range h.products {
		out = append(out, *p)
	}
	return out
}

// ByCategory returns products in a given category.
func (h *Hub) ByCategory(category string) []Product {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []Product
	for _, p := range h.products {
		if p.Category == category {
			out = append(out, *p)
		}
	}
	return out
}

// Healthy returns all products with healthy status.
func (h *Hub) Healthy() []Product {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []Product
	for _, p := range h.products {
		if p.Status == StatusHealthy || p.Status == StatusEmbedded {
			out = append(out, *p)
		}
	}
	return out
}

// Count returns total registered products.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.products)
}

// StartHealthChecks begins periodic health checking.
func (h *Hub) StartHealthChecks(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		h.checkAll()
		for {
			select {
			case <-ticker.C:
				h.checkAll()
			case <-h.stopCh:
				return
			}
		}
	}()
}

// Stop halts health checking.
func (h *Hub) Stop() {
	close(h.stopCh)
}

func (h *Hub) checkAll() {
	h.mu.RLock()
	products := make([]*Product, 0, len(h.products))
	for _, p := range h.products {
		products = append(products, p)
	}
	h.mu.RUnlock()

	for _, p := range products {
		if p.Status == StatusEmbedded || p.HealthURL == "" {
			continue
		}
		status := h.checkHealth(p.HealthURL)
		h.mu.Lock()
		p.Status = status
		p.LastCheck = time.Now()
		h.mu.Unlock()
	}
}

func (h *Hub) checkHealth(url string) Status {
	resp, err := h.client.Get(url)
	if err != nil {
		return StatusDown
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return StatusHealthy
	}
	if resp.StatusCode < 500 {
		return StatusDegraded
	}
	return StatusDown
}

// RegisterAll adds all 17 Stockyard products with their metadata.
func RegisterAll(h *Hub) {
	products := []Product{
		// Lifecycle
		{ID: "iron", Name: "Iron", Description: "Burns specs into working code", Category: "lifecycle",
			Capabilities: []string{"spec.created"}, Emits: []string{"code.generated", "tests.passed", "tests.failed", "build.failed"},
			Status: StatusEmbedded},
		{ID: "stampede", Name: "Stampede", Description: "AI synthetic user testing", Category: "lifecycle",
			Capabilities: []string{"code.generated", "deploy.completed"}, Emits: []string{"tests.passed", "tests.failed", "quality.scored"},
			Status: StatusEmbedded},
		{ID: "verdikt", Name: "Verdikt", Description: "AI output quality judge", Category: "lifecycle",
			Capabilities: []string{"request.completed"}, Emits: []string{"quality.scored", "quality.degraded", "drift.detected"},
			Status: StatusEmbedded},

		// Runtime
		{ID: "bid", Name: "Bid", Description: "Real-time LLM model exchange", Category: "runtime",
			Capabilities: []string{"request.received"}, Emits: []string{"auction.complete", "request.completed", "provider.down"},
			Status: StatusEmbedded},
		{ID: "morph", Name: "Morph", Description: "Universal API translator", Category: "runtime",
			Capabilities: []string{"request.received"}, Emits: []string{"request.completed"},
			Status: StatusEmbedded},
		{ID: "grain", Name: "Grain", Description: "Decisions as first-class objects", Category: "runtime",
			Capabilities: []string{"request.received"}, Emits: []string{"decision.made", "decision.override"},
			Status: StatusEmbedded},

		// Quality
		{ID: "fault", Name: "Fault", Description: "Self-healing errors", Category: "quality",
			Capabilities: []string{"error.detected", "panic.caught"}, Emits: []string{"error.diagnosed", "patch.generated", "patch.applied"},
			Status: StatusEmbedded},
		{ID: "doubt", Name: "Doubt", Description: "Confidence scores for code", Category: "quality",
			Capabilities: []string{"request.completed", "error.detected"}, Emits: []string{"confidence.low"},
			Status: StatusEmbedded},
		{ID: "hollow", Name: "Hollow", Description: "Negative space analysis", Category: "quality",
			Capabilities: []string{"code.generated", "tests.failed"}, Emits: []string{"gap.found"},
			Status: StatusEmbedded},

		// Infrastructure
		{ID: "spine", Name: "Spine", Description: "Self-operating infrastructure", Category: "infra",
			Capabilities: []string{"pressure.high", "error.detected"}, Emits: []string{"scale.up", "scale.down", "pressure.high", "pressure.normal"},
			Status: StatusEmbedded},
		{ID: "tide", Name: "Tide", Description: "Metabolic feature control", Category: "infra",
			Capabilities: []string{"pressure.high", "pressure.normal"}, Emits: []string{"feature.hibernated", "feature.woke"},
			Status: StatusEmbedded},
		{ID: "replay", Name: "Replay", Description: "Time-travel debugging", Category: "infra",
			Capabilities: []string{"request.received", "request.completed", "error.detected"}, Emits: []string{"state.recorded"},
			Status: StatusEmbedded},

		// Insight
		{ID: "seance", Name: "Séance", Description: "Talk to production", Category: "insight",
			Capabilities: []string{"query.answered"}, Emits: []string{"query.answered"},
			Status: StatusEmbedded},
		{ID: "trailhead", Name: "Trailhead", Description: "Codebase knowledge engine", Category: "insight",
			Capabilities: []string{"code.generated", "annotation.added"}, Emits: []string{"index.complete", "query.answered"},
			Status: StatusEmbedded},
		{ID: "echo", Name: "Echo", Description: "Evolutionary code memory", Category: "insight",
			Capabilities: []string{"code.generated", "patch.applied"}, Emits: []string{"annotation.added"},
			Status: StatusEmbedded},
		{ID: "fossil", Name: "Fossil", Description: "Dead code archaeology", Category: "insight",
			Capabilities: []string{"code.generated"}, Emits: []string{"deadcode.found"},
			Status: StatusEmbedded},
		{ID: "prism", Name: "Prism", Description: "User cognitive mapping", Category: "insight",
			Capabilities: []string{"request.completed"}, Emits: []string{"user.confused"},
			Status: StatusEmbedded},
	}

	for _, p := range products {
		h.Register(p)
	}
}

// Summary returns a quick overview.
type Summary struct {
	Total     int            `json:"total"`
	Healthy   int            `json:"healthy"`
	Degraded  int            `json:"degraded"`
	Down      int            `json:"down"`
	Categories map[string]int `json:"categories"`
}

func (h *Hub) Summary() Summary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	s := Summary{
		Total:      len(h.products),
		Categories: map[string]int{},
	}
	for _, p := range h.products {
		switch p.Status {
		case StatusHealthy, StatusEmbedded:
			s.Healthy++
		case StatusDegraded:
			s.Degraded++
		case StatusDown:
			s.Down++
		}
		s.Categories[p.Category]++
	}
	return s
}

// FindByCapability returns products that can handle a given event type.
func (h *Hub) FindByCapability(eventType string) []Product {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var out []Product
	for _, p := range h.products {
		for _, cap := range p.Capabilities {
			if cap == eventType {
				out = append(out, *p)
				break
			}
		}
	}
	return out
}

// DependencyGraph shows which products feed into which.
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	EventType string `json:"event_type"`
}

func (h *Hub) DependencyGraph() []Edge {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var edges []Edge
	emitters := map[string][]string{} // event -> producers

	for _, p := range h.products {
		for _, e := range p.Emits {
			emitters[e] = append(emitters[e], p.ID)
		}
	}

	for _, p := range h.products {
		for _, cap := range p.Capabilities {
			for _, producer := range emitters[cap] {
				if producer != p.ID {
					edges = append(edges, Edge{From: producer, To: p.ID, EventType: cap})
				}
			}
		}
	}

	return edges
}

// GraphViz returns a DOT representation of the product dependency graph.
func (h *Hub) GraphViz() string {
	edges := h.DependencyGraph()
	dot := "digraph stockyard {\n  rankdir=LR;\n  node [shape=box, style=filled, fillcolor=\"#2a2318\", fontcolor=\"#f5f0e8\", color=\"#8b4513\"];\n\n"

	seen := map[string]bool{}
	for _, e := range edges {
		key := fmt.Sprintf("%s->%s", e.From, e.To)
		if seen[key] {
			continue
		}
		seen[key] = true
		dot += fmt.Sprintf("  %s -> %s [label=\"%s\", fontsize=9, color=\"#d4a574\"];\n", e.From, e.To, e.EventType)
	}

	dot += "}\n"
	return dot
}
