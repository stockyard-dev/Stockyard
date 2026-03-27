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

// RegisterAll populates the registry with all 29 platform products.
func (r *ProductRegistry) RegisterAll(activeTier Tier) {
	type productDef struct {
		id, name, desc, category string
	}
	defs := []productDef{
		{"bid", "Auction", "Dynamic model bidding — providers compete on price", "range"},
		{"replay", "Lasso", "Request replay — re-run traces against different models", "fence"},
		{"doubt", "Doubt", "Hallucination detection — flag uncertain outputs", "herd"},
		{"verdikt", "Verdikt", "Quality gates — block bad responses before they ship", "trail"},
		{"stampede", "Stampede", "Load testing — flood your stack with synthetic traffic", "trail"},
		{"fault", "Fault", "Chaos engineering — inject latency, errors, rate limits", "herd"},
		{"morph", "Morph", "Request transformation — rewrite, augment, reshape in flight", "range"},
		{"spine", "Spine", "Health probes, readiness checks, platform diagnostics", "fence"},
		{"hollow", "Hollow", "Shadow testing — compare models silently", "herd"},
		{"seance", "Séance", "Resurrect and replay historical conversations", "scout"},
		{"grain", "Grain", "Fine-grained access control — per-key, per-model permissions", "range"},
		{"echo", "Echo", "Response monitoring — track output changes across versions", "scout"},
		{"tide", "Tide", "Schema migration and platform lifecycle management", "fence"},
		{"fossil", "Fossil", "Deep archival with compression and retrieval", "scout"},
		{"prism", "Prism", "Multi-angle analysis — cost, quality, speed perspectives", "scout"},
		{"trailhead", "Trailhead", "Onboarding intelligence — optimize the first experience", "scout"},
		{"iron", "Iron", "Hardening and compliance — security policies platform-wide", "trail"},
		{"orchestrator", "Ramrod", "Orchestration — coordinate workflows across products", "trail"},

		// ── New Products ──
		{"relic", "Relic", "Provenance tracking — full lineage from prompt to response", "herd"},
		{"breed", "Breed", "Genetic prompt optimization — evolve, crossover, tournament", "trail"},
		{"fossilrec", "Fossil Record", "Historical analysis — model and cost trends over time", "herd"},
		{"phantom", "Phantom", "Persona-based testing — synthetic users probe for weaknesses", "herd"},
		{"feral", "Feral", "Red-team engine — 29 attack probes, 9 categories", "herd"},
		{"tidepool", "Tide Pool", "Micro-analytics — small-scale pattern detection", "scout"},
		{"crucible", "Crucible", "Confidence scoring — track model certainty over time", "herd"},
		{"cortex", "Cortex", "Platform memory — live stats, trace enrichment, institutional knowledge", "scout"},
		{"mycelium", "Mycelium", "Insight extraction — cross-instance pattern surfacing", "fence"},
		{"spore", "Spore", "Pattern replication with cap and retirement", "fence"},
		{"molt", "Molt", "Auto-shed unused modules — prune dead weight", "fence"},
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

		// Seed platform_product_state so Cortex/Molt can query product counts
		if r.db != nil {
			enabledInt := 0
			if active {
				enabledInt = 1
			}
			r.db.Exec(`INSERT INTO platform_product_state (product_id, enabled, updated_at)
				VALUES (?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT(product_id) DO NOTHING`,
				d.id, enabledInt)
		}
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
