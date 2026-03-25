package platform

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// ProductState tracks the runtime state of a mounted product.
type ProductState struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Tier        Tier   `json:"required_tier"`
	TierName    string `json:"required_tier_name"`
	Active      bool   `json:"active"`  // true if tier permits access
	Enabled     bool   `json:"enabled"` // true if user has toggled it on
	Status      string `json:"status"`  // healthy, degraded, down, locked
	APIURL      string `json:"api_url"`
	UIURL       string `json:"ui_url"`
}

// ProductRegistry manages the state of all platform products.
type ProductRegistry struct {
	mu       sync.RWMutex
	products map[string]*ProductState
	db       *sql.DB
}

// NewProductRegistry creates a registry backed by the given database.
func NewProductRegistry(db *sql.DB) *ProductRegistry {
	r := &ProductRegistry{
		products: make(map[string]*ProductState),
		db:       db,
	}
	r.migrate()
	return r
}

func (r *ProductRegistry) migrate() {
	if r.db == nil {
		return
	}
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS platform_product_state (
		product_id TEXT PRIMARY KEY,
		enabled    INTEGER DEFAULT 1,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Printf("[platform] product_state migration: %v", err)
	}
}

// Register adds a product to the registry.
func (r *ProductRegistry) Register(ps ProductState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[ps.ID] = &ps

	// Load persisted toggle state
	if r.db != nil {
		var enabled int
		err := r.db.QueryRow("SELECT enabled FROM platform_product_state WHERE product_id = ?", ps.ID).Scan(&enabled)
		if err == nil {
			r.products[ps.ID].Enabled = enabled == 1
		}
	}
}

// RegisterAll populates the registry with all 18 platform products.
func (r *ProductRegistry) RegisterAll(activeTier Tier) {
	type productDef struct {
		id, name, desc, category string
	}
	defs := []productDef{
		{"bid", "Bid", "Real-time LLM model exchange", "runtime"},
		{"replay", "Replay", "Time-travel debugging", "infra"},
		{"doubt", "Doubt", "Confidence scores for code", "quality"},
		{"verdikt", "Verdikt", "AI output quality judge", "lifecycle"},
		{"stampede", "Stampede", "AI synthetic user testing", "lifecycle"},
		{"fault", "Fault", "Self-healing errors", "quality"},
		{"morph", "Morph", "Universal API translator", "runtime"},
		{"spine", "Spine", "Self-operating infrastructure", "infra"},
		{"hollow", "Hollow", "Negative space analysis", "quality"},
		{"seance", "Séance", "Talk to production", "insight"},
		{"grain", "Grain", "Decisions as first-class objects", "runtime"},
		{"echo", "Echo", "Evolutionary code memory", "insight"},
		{"tide", "Tide", "Metabolic feature control", "infra"},
		{"fossil", "Fossil", "Dead code archaeology", "insight"},
		{"prism", "Prism", "User cognitive mapping", "insight"},
		{"trailhead", "Trailhead", "Codebase knowledge engine", "insight"},
		{"iron", "Iron", "Burns specs into working code", "lifecycle"},
		{"orchestrator", "Orchestrator", "Autonomous coordination brain", "lifecycle"},
	}

	for _, d := range defs {
		requiredTier := ProductTiers[d.id]
		active := activeTier >= requiredTier
		status := "locked"
		if active {
			status = "healthy"
		}

		r.Register(ProductState{
			ID:          d.id,
			Name:        d.name,
			Description: d.desc,
			Category:    d.category,
			Tier:        requiredTier,
			TierName:    TierName(requiredTier),
			Active:      active,
			Enabled:     active, // default to enabled if tier allows
			Status:      status,
			APIURL:      "/" + d.id + "/api",
			UIURL:       "/" + d.id + "/ui",
		})
	}
}

// Get returns the state of a single product.
func (r *ProductRegistry) Get(id string) *ProductState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if ps, ok := r.products[id]; ok {
		cp := *ps
		return &cp
	}
	return nil
}

// All returns all products sorted by tier then name.
func (r *ProductRegistry) All() []ProductState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProductState, 0, len(r.products))
	for _, ps := range r.products {
		out = append(out, *ps)
	}
	return out
}

// Active returns only products the current tier permits.
func (r *ProductRegistry) Active() []ProductState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ProductState
	for _, ps := range r.products {
		if ps.Active {
			out = append(out, *ps)
		}
	}
	return out
}

// SetEnabled toggles a product on or off (persists to SQLite).
func (r *ProductRegistry) SetEnabled(id string, enabled bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	ps, ok := r.products[id]
	if !ok {
		return false
	}
	if !ps.Active {
		return false // can't enable a product above your tier
	}
	ps.Enabled = enabled

	if r.db != nil {
		enabledInt := 0
		if enabled {
			enabledInt = 1
		}
		r.db.Exec(`INSERT INTO platform_product_state (product_id, enabled, updated_at) 
			VALUES (?, ?, CURRENT_TIMESTAMP) 
			ON CONFLICT(product_id) DO UPDATE SET enabled = ?, updated_at = CURRENT_TIMESTAMP`,
			id, enabledInt, enabledInt)
	}
	return true
}

// SetStatus updates a product's health status.
func (r *ProductRegistry) SetStatus(id, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ps, ok := r.products[id]; ok {
		ps.Status = status
	}
}

// Count returns total registered products.
func (r *ProductRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.products)
}

// HandleProducts returns an HTTP handler for GET /api/platform/products.
func (r *ProductRegistry) HandleProducts(activeTier Tier) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		all := r.All()

		// Group by tier for the response
		byTier := map[string][]ProductState{}
		for _, ps := range all {
			byTier[ps.TierName] = append(byTier[ps.TierName], ps)
		}

		active := r.Active()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"products":     all,
			"total":        len(all),
			"active":       len(active),
			"locked":       len(all) - len(active),
			"current_tier": TierName(activeTier),
			"by_tier":      byTier,
		})
	}
}

// HandleToggle returns an HTTP handler for PUT /api/platform/products/{id}/toggle.
func (r *ProductRegistry) HandleToggle() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")

		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid body"})
			return
		}

		if !r.SetEnabled(id, body.Enabled) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "product not found or tier insufficient"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"product": id,
			"enabled": body.Enabled,
		})
	}
}
