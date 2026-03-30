// Package proxy implements App 1: Proxy — core reverse-proxy, middleware chain, provider dispatch.
// The actual proxy engine lives in internal/engine + internal/proxy. This app package
// provides the management API and module configuration layer.
package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/features"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/toggle"
)

// App implements platform.App for the Proxy application.
type App struct {
	conn       *sql.DB
	toggle     *toggle.Registry
	audit      func(string, string, string, string, any)
	failover   *features.FailoverRouter
	modelAlias *features.ModelAliasState
}

// New creates a new Proxy app instance.
func New(conn *sql.DB) *App {
	return &App{conn: conn}
}

// SetToggleRegistry connects the proxy app to the runtime middleware toggle.
func (a *App) SetToggleRegistry(reg *toggle.Registry) {
	a.toggle = reg
}

// SetAuditor wires the trust audit function for recording admin actions.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

// SetFailoverRouter connects the proxy app to the failover router for circuit breaker management.
func (a *App) SetFailoverRouter(router *features.FailoverRouter) {
	a.failover = router
}

// SetModelAlias connects the proxy app to the model alias state for runtime management.
func (a *App) SetModelAlias(ma *features.ModelAliasState) {
	a.modelAlias = ma
	// Load any persisted aliases from DB
	if a.conn != nil {
		rows, err := a.conn.Query(`SELECT alias, model FROM proxy_aliases`)
		if err != nil {
			log.Printf("[proxy] warning: could not load persisted aliases: %v", err)
			return
		}
		defer rows.Close()
		loaded := 0
		for rows.Next() {
			var alias, model string
			if err := rows.Scan(&alias, &model); err == nil {
				ma.Set(alias, model)
				loaded++
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		if loaded > 0 {
			log.Printf("[proxy] loaded %d persisted aliases from DB", loaded)
		}
	}
}

func (a *App) auditEvent(action, resource string, detail any) {
	if a.audit != nil {
		go a.audit("admin_action", "proxy_admin", resource, action, detail)
	}
}

func (a *App) Name() string        { return "proxy" }
func (a *App) Description() string { return "Core reverse-proxy, middleware chain, provider dispatch" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(proxySchema)
	if err != nil {
		return err
	}
	log.Printf("[proxy] migrations applied")
	return nil
}

const proxySchema = `
CREATE TABLE IF NOT EXISTS proxy_modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    enabled INTEGER NOT NULL DEFAULT 1,
    config_json TEXT DEFAULT '{}',
    priority INTEGER DEFAULT 100,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    base_url TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    health_check_url TEXT DEFAULT '',
    last_check TEXT DEFAULT '',
    latency_ms INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    config_json TEXT DEFAULT '{}',
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'POST',
    provider TEXT NOT NULL,
    model TEXT DEFAULT '',
    middleware_json TEXT DEFAULT '[]',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_aliases (
    alias TEXT PRIMARY KEY NOT NULL,
    model TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/proxy/modules", a.handleListModules)
	mux.HandleFunc("GET /api/proxy/modules/{name}", a.handleGetModule)
	mux.HandleFunc("PUT /api/proxy/modules/{name}", a.handleUpdateModule)
	mux.HandleFunc("POST /api/proxy/modules/bulk", a.handleBulkToggle)
	mux.HandleFunc("GET /api/proxy/providers", a.handleListProviders)
	mux.HandleFunc("GET /api/proxy/providers/health", a.handleHealthCheckAll)
	mux.HandleFunc("POST /api/proxy/providers/{name}/check", a.handleCheckProvider)
	mux.HandleFunc("GET /api/proxy/routes", a.handleListRoutes)
	mux.HandleFunc("GET /api/proxy/chain", a.handleChain)
	mux.HandleFunc("GET /api/proxy/status", a.handleStatus)
	mux.HandleFunc("GET /api/proxy/pricing", a.handlePricing)
	mux.HandleFunc("POST /api/proxy/estimate", a.handleEstimate)
	mux.HandleFunc("GET /api/proxy/recommendations", a.handleRecommendations)
	mux.HandleFunc("GET /api/proxy/breakers", a.handleBreakerStates)
	mux.HandleFunc("POST /api/proxy/breakers/reset", a.handleResetAllBreakers)
	mux.HandleFunc("POST /api/proxy/breakers/{name}/reset", a.handleResetBreaker)
	mux.HandleFunc("GET /api/proxy/aliases", a.handleListAliases)
	mux.HandleFunc("PUT /api/proxy/aliases", a.handleSetAlias)
	mux.HandleFunc("DELETE /api/proxy/aliases/{alias}", a.handleDeleteAlias)
	mux.HandleFunc("GET /api/proxy/aliases/stats", a.handleAliasStats)
	log.Printf("[proxy] routes registered")
}

func (a *App) handleListModules(w http.ResponseWriter, r *http.Request) {
	// Optional query filters
	category := r.URL.Query().Get("category")
	enabledFilter := r.URL.Query().Get("enabled")

	query := "SELECT name, category, enabled, config_json, priority FROM proxy_modules"
	var args []any
	var where []string
	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	if enabledFilter == "true" {
		where = append(where, "enabled = 1")
	} else if enabledFilter == "false" {
		where = append(where, "enabled = 0")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY priority"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"modules": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	// Build set of modules actually in the live chain
	chainSet := make(map[string]bool)
	if a.toggle != nil {
		chainSet = a.toggle.KnownModules()
	}

	var modules []map[string]any
	for rows.Next() {
		var name, cat, configJSON string
		var enabled, priority int
		if err := rows.Scan(&name, &cat, &enabled, &configJSON, &priority); err != nil {
			continue
		}
		var cfg any
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			log.Printf("[proxy] json parse error: %v", err)
		}
		modules = append(modules, map[string]any{
			"name": name, "category": cat, "enabled": enabled == 1,
			"config": cfg, "priority": priority, "in_chain": chainSet[name],
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	writeJSON(w, map[string]any{"modules": modules, "count": len(modules)})
}

func (a *App) handleGetModule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	row := a.conn.QueryRow("SELECT name, category, enabled, config_json, priority, updated_at FROM proxy_modules WHERE name = ?", name)
	var modName, cat, configJSON, updatedAt string
	var enabled, priority int
	if err := row.Scan(&modName, &cat, &enabled, &configJSON, &priority, &updatedAt); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "module not found", "name": name})
		return
	}
	var cfg any
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		log.Printf("[proxy] json parse error: %v", err)
	}

	inChain := false
	if a.toggle != nil {
		known := a.toggle.KnownModules()
		inChain = known[name]
	}

	writeJSON(w, map[string]any{
		"name": modName, "category": cat, "enabled": enabled == 1,
		"config": cfg, "priority": priority, "updated_at": updatedAt,
		"in_chain": inChain,
	})
}

func (a *App) handleUpdateModule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Enabled  *bool `json:"enabled"`
		Config   any   `json:"config"`
		Priority *int  `json:"priority"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Enabled != nil {
		enabled := 0
		if *req.Enabled {
			enabled = 1
		}
		a.conn.Exec("UPDATE proxy_modules SET enabled = ?, updated_at = ? WHERE name = ?", enabled, time.Now().Format(time.RFC3339), name)
		// Toggle live middleware chain
		if a.toggle != nil {
			a.toggle.Set(name, *req.Enabled)
		}
	}
	if req.Config != nil {
		j, marshalErr := json.Marshal(req.Config)
		if marshalErr != nil {
			j = []byte("{}")
		}
		a.conn.Exec("UPDATE proxy_modules SET config_json = ?, updated_at = ? WHERE name = ?", string(j), time.Now().Format(time.RFC3339), name)
	}
	if req.Priority != nil {
		a.conn.Exec("UPDATE proxy_modules SET priority = ?, updated_at = ? WHERE name = ?", *req.Priority, time.Now().Format(time.RFC3339), name)
	}

	writeJSON(w, map[string]string{"status": "updated", "module": name})
	a.auditEvent("module_updated", name, map[string]any{
		"enabled": req.Enabled, "has_config": req.Config != nil, "has_priority": req.Priority != nil,
	})
}

func (a *App) handleBulkToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Modules  []string `json:"modules"`  // specific module names
		Category string   `json:"category"` // or toggle by category
		Enabled  bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid request body"})
		return
	}

	now := time.Now().Format(time.RFC3339)
	var affected int64

	if len(req.Modules) > 0 {
		// Toggle specific modules
		for _, name := range req.Modules {
			enabled := 0
			if req.Enabled {
				enabled = 1
			}
			res, err := a.conn.Exec("UPDATE proxy_modules SET enabled = ?, updated_at = ? WHERE name = ?", enabled, now, name)
			if err == nil {
				n, _ := res.RowsAffected()
				affected += n
			}
			if a.toggle != nil {
				a.toggle.Set(name, req.Enabled)
			}
		}
	} else if req.Category != "" {
		// Toggle all modules in a category
		enabled := 0
		if req.Enabled {
			enabled = 1
		}
		res, err := a.conn.Exec("UPDATE proxy_modules SET enabled = ?, updated_at = ? WHERE category = ?", enabled, now, req.Category)
		if err == nil {
			affected, _ = res.RowsAffected()
		}
		// Update toggle registry for all modules in category
		if a.toggle != nil {
			rows, _ := a.conn.Query("SELECT name FROM proxy_modules WHERE category = ?", req.Category)
			if rows != nil {
				defer rows.Close()
				for rows.Next() {
					var name string
					if err := rows.Scan(&name); err != nil {
						continue
					}
					a.toggle.Set(name, req.Enabled)
				}
				if err := rows.Err(); err != nil {
					log.Printf("[db] rows iteration error: %v", err)
				}
			}
		}
	} else {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "provide 'modules' array or 'category' string"})
		return
	}

	writeJSON(w, map[string]any{"status": "updated", "affected": affected, "enabled": req.Enabled})
	a.auditEvent("bulk_toggle", "proxy_modules", map[string]any{
		"enabled": req.Enabled, "affected": affected, "category": req.Category,
	})
}

func (a *App) handleChain(w http.ResponseWriter, r *http.Request) {
	// Report which modules are actually in the live middleware chain
	// and their current toggle state
	chainSet := make(map[string]bool)
	if a.toggle != nil {
		chainSet = a.toggle.KnownModules()
	}

	type chainEntry struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	var chain []chainEntry
	for name, enabled := range chainSet {
		chain = append(chain, chainEntry{Name: name, Enabled: enabled})
	}

	// Also get categories from DB for the chain modules
	catMap := make(map[string]string)
	rows, err := a.conn.Query("SELECT name, category FROM proxy_modules")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var n, c string
			if err := rows.Scan(&n, &c); err != nil {
				continue
			}
			catMap[n] = c
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}

	type richEntry struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Enabled  bool   `json:"enabled"`
		InChain  bool   `json:"in_chain"`
	}
	var rich []richEntry
	for name, enabled := range chainSet {
		rich = append(rich, richEntry{
			Name: name, Category: catMap[name], Enabled: enabled, InChain: true,
		})
	}

	writeJSON(w, map[string]any{
		"chain":         rich,
		"chain_length":  len(rich),
		"total_modules": len(catMap),
	})
}

func (a *App) handleListProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT name, base_url, status, latency_ms, error_count, request_count, last_check FROM proxy_providers ORDER BY name")
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer rows.Close()

	var providers []map[string]any
	for rows.Next() {
		var name, baseURL, status, lastCheck string
		var latency, errors, requests int
		if err := rows.Scan(&name, &baseURL, &status, &latency, &errors, &requests, &lastCheck); err != nil {
			continue
		}
		providers = append(providers, map[string]any{
			"name": name, "base_url": baseURL, "status": status,
			"latency_ms": latency, "error_count": errors,
			"request_count": requests, "last_check": lastCheck,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	writeJSON(w, map[string]any{"providers": providers, "count": len(providers)})
}

func (a *App) handleCheckProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, map[string]string{"error": "provider name required"})
		return
	}

	// Look up the provider's base URL
	var baseURL string
	err := a.conn.QueryRow("SELECT base_url FROM proxy_providers WHERE name = ?", name).Scan(&baseURL)
	if err != nil {
		writeJSON(w, map[string]string{"error": "provider not found", "provider": name})
		return
	}

	now := time.Now().Format(time.RFC3339)
	status := "active"

	// Actually probe the provider if a base URL is configured
	if baseURL != "" {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(baseURL)
		if err != nil {
			status = "unreachable"
			log.Printf("[proxy] health check for %s failed: %v", name, err)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				status = "degraded"
			}
		}
	}

	a.conn.Exec("UPDATE proxy_providers SET last_check = ?, status = ? WHERE name = ?", now, status, name)
	writeJSON(w, map[string]string{"status": status, "provider": name, "checked_at": now})
}

// handleHealthCheckAll checks all configured providers concurrently and returns their status.
func (a *App) handleHealthCheckAll(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT name, base_url FROM proxy_providers ORDER BY name")
	if err != nil {
		writeJSON(w, map[string]any{"providers": []any{}, "error": "query failed"})
		return
	}
	defer rows.Close()

	type provInfo struct {
		name    string
		baseURL string
	}
	var providers []provInfo
	for rows.Next() {
		var p provInfo
		if err := rows.Scan(&p.name, &p.baseURL); err != nil {
			continue
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	type result struct {
		Provider  string `json:"provider"`
		Status    string `json:"status"`
		LatencyMs int64  `json:"latency_ms"`
		Error     string `json:"error,omitempty"`
	}

	results := make([]result, len(providers))
	var wg sync.WaitGroup

	for i, p := range providers {
		wg.Add(1)
		go func(idx int, prov provInfo) {
			defer wg.Done()
			res := result{Provider: prov.name}

			if prov.baseURL == "" {
				res.Status = "unconfigured"
				results[idx] = res
				return
			}

			start := time.Now()
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(prov.baseURL)
			latency := time.Since(start).Milliseconds()
			res.LatencyMs = latency

			if err != nil {
				res.Status = "unreachable"
				res.Error = fmt.Sprintf("%v", err)
			} else {
				resp.Body.Close()
				if resp.StatusCode >= 500 {
					res.Status = "degraded"
					res.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				} else {
					res.Status = "ok"
				}
			}

			// Update provider status in DB
			now := time.Now().Format(time.RFC3339)
			a.conn.Exec("UPDATE proxy_providers SET last_check = ?, status = ?, latency_ms = ? WHERE name = ?",
				now, res.Status, latency, prov.name)

			results[idx] = res
		}(i, p)
	}

	wg.Wait()

	// Count stats
	healthy := 0
	for _, r := range results {
		if r.Status == "ok" {
			healthy++
		}
	}

	writeJSON(w, map[string]any{
		"providers":  results,
		"total":      len(results),
		"healthy":    healthy,
		"checked_at": time.Now().Format(time.RFC3339),
	})
}

func (a *App) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT path, method, provider, model, enabled FROM proxy_routes ORDER BY path")
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer rows.Close()

	var routes []map[string]any
	for rows.Next() {
		var path, method, prov, model string
		var enabled int
		if err := rows.Scan(&path, &method, &prov, &model, &enabled); err != nil {
			continue
		}
		routes = append(routes, map[string]any{
			"path": path, "method": method, "provider": prov,
			"model": model, "enabled": enabled == 1,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	writeJSON(w, map[string]any{"routes": routes})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	var moduleCount, enabledCount, providerCount, routeCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_modules").Scan(&moduleCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_modules WHERE enabled = 1").Scan(&enabledCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_providers").Scan(&providerCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_routes").Scan(&routeCount)

	// Count modules actually in the live middleware chain
	var chainCount int
	if a.toggle != nil {
		for _, enabled := range a.toggle.KnownModules() {
			if enabled {
				chainCount++
			}
		}
	}

	writeJSON(w, map[string]any{
		"app":             "proxy",
		"status":          "running",
		"total_modules":   moduleCount,
		"enabled_modules": enabledCount,
		"live_chain":      chainCount,
		"providers":       providerCount,
		"routes":          routeCount,
	})
}

// handlePricing returns the full pricing table for client-side cost estimation.
func (a *App) handlePricing(w http.ResponseWriter, r *http.Request) {
	table := provider.ListPricing()
	type entry struct {
		Model    string  `json:"model"`
		Provider string  `json:"provider"`
		Input    float64 `json:"input_per_m"`
		Output   float64 `json:"output_per_m"`
	}
	var entries []entry
	for model, p := range table {
		entries = append(entries, entry{Model: model, Provider: p.Provider, Input: p.Input, Output: p.Output})
	}
	writeJSON(w, map[string]any{"pricing": entries, "count": len(entries)})
}

// handleEstimate estimates the cost of a request before sending it.
func (a *App) handleEstimate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model     string `json:"model"`
		InputText string `json:"input_text"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid request"})
		return
	}
	if req.Model == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "model required"})
		return
	}

	// Estimate tokens: ~1 token per 4 characters
	estInputTokens := len(req.InputText) / 4
	if estInputTokens < 1 {
		estInputTokens = 1
	}
	estOutputTokens := req.MaxTokens
	if estOutputTokens <= 0 {
		estOutputTokens = 2048
	}

	pricing, hasPricing := provider.GetPricing(req.Model)
	inputCost := provider.CalculateCost(req.Model, estInputTokens, 0)
	outputCost := provider.CalculateCost(req.Model, 0, estOutputTokens)
	totalCost := inputCost + outputCost

	result := map[string]any{
		"model":             req.Model,
		"est_input_tokens":  estInputTokens,
		"est_output_tokens": estOutputTokens,
		"input_cost_usd":    inputCost,
		"output_cost_usd":   outputCost,
		"total_cost_usd":    totalCost,
		"total_cost_cents":  int64(totalCost * 100),
		"has_pricing":       hasPricing,
	}
	if hasPricing {
		result["input_per_m"] = pricing.Input
		result["output_per_m"] = pricing.Output
		result["provider"] = pricing.Provider
	}

	writeJSON(w, result)
}

// handleRecommendations analyzes recent trace data and suggests module optimizations.
func (a *App) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	type recommendation struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Reason   string `json:"reason"`
		Module   string `json:"module"`
		Priority string `json:"priority"` // high, medium, low
	}
	var recs []recommendation

	// Count recent requests
	var totalRequests int
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', '-7 days')").Scan(&totalRequests)

	if totalRequests < 10 {
		writeJSON(w, map[string]any{"recommendations": recs, "total_requests_analyzed": totalRequests, "message": "Need at least 10 requests in the last 7 days for recommendations"})
		return
	}

	// 1. Check for duplicate prompts → recommend cache
	var dupCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM (
		SELECT metadata_json, COUNT(*) as cnt FROM observe_traces
		WHERE created_at >= datetime('now', '-7 days') AND status = 'ok'
		GROUP BY metadata_json HAVING cnt > 3
	)`).Scan(&dupCount)

	var cacheEnabled int
	a.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = 'cache'").Scan(&cacheEnabled)

	if dupCount > 0 && cacheEnabled != 1 {
		recs = append(recs, recommendation{
			ID:       "enable-cache",
			Title:    "Enable Response Cache",
			Reason:   fmt.Sprintf("Found %d repeated prompt patterns in the last 7 days. Caching could reduce costs and latency.", dupCount),
			Module:   "cache",
			Priority: "high",
		})
	}

	// 2. Check for single provider usage → recommend failover
	var providerCount int
	a.conn.QueryRow(`SELECT COUNT(DISTINCT provider) FROM observe_traces WHERE created_at >= datetime('now', '-7 days') AND provider != ''`).Scan(&providerCount)

	var failoverEnabled int
	a.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = 'failover'").Scan(&failoverEnabled)

	if providerCount <= 1 && failoverEnabled != 1 {
		recs = append(recs, recommendation{
			ID:       "enable-failover",
			Title:    "Configure Provider Failover",
			Reason:   "All requests are going to a single provider. Adding failover ensures uptime if that provider has an outage.",
			Module:   "failover",
			Priority: "medium",
		})
	}

	// 3. Check for high-cost requests → recommend cost caps
	var highCostCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', '-7 days') AND cost_usd > 0.10`).Scan(&highCostCount)

	var capsEnabled int
	a.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = 'caps'").Scan(&capsEnabled)

	if highCostCount > 5 && capsEnabled != 1 {
		recs = append(recs, recommendation{
			ID:       "enable-caps",
			Title:    "Set Spend Caps",
			Reason:   fmt.Sprintf("%d requests in the last week cost over $0.10 each. Spend caps prevent runaway costs.", highCostCount),
			Module:   "caps",
			Priority: "high",
		})
	}

	// 4. Check for high token counts → recommend promptslim
	var highTokenCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', '-7 days') AND tokens_in > 4000`).Scan(&highTokenCount)

	var promptslimEnabled int
	a.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = 'promptslim'").Scan(&promptslimEnabled)

	if highTokenCount > 5 && promptslimEnabled != 1 {
		recs = append(recs, recommendation{
			ID:       "enable-promptslim",
			Title:    "Enable Prompt Compression",
			Reason:   fmt.Sprintf("%d requests used 4000+ input tokens. PromptSlim can reduce token usage by 20-40%%.", highTokenCount),
			Module:   "promptslim",
			Priority: "medium",
		})
	}

	// 5. Check for expensive models on simple/short tasks → recommend tierdrop
	var expensiveSimpleCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces
		WHERE created_at >= datetime('now', '-7 days')
		AND tokens_in < 500 AND tokens_out < 200
		AND (model LIKE '%gpt-4o%' OR model LIKE '%claude-opus%' OR model LIKE '%o1%' OR model LIKE '%o3%')
		AND model NOT LIKE '%mini%' AND model NOT LIKE '%nano%'`).Scan(&expensiveSimpleCount)

	var tierdropEnabled int
	a.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = 'tierdrop'").Scan(&tierdropEnabled)

	if expensiveSimpleCount > 10 && tierdropEnabled != 1 {
		recs = append(recs, recommendation{
			ID:       "enable-tierdrop",
			Title:    "Auto-Downgrade Simple Requests",
			Reason:   fmt.Sprintf("%d short requests used expensive models. TierDrop can route simple tasks to cheaper models automatically.", expensiveSimpleCount),
			Module:   "tierdrop",
			Priority: "medium",
		})
	}

	// 6. Check error rate → recommend retrypilot
	var errorCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', '-7 days') AND status = 'error'`).Scan(&errorCount)

	errorRate := float64(errorCount) / float64(totalRequests)
	var retryEnabled int
	a.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = 'retrypilot'").Scan(&retryEnabled)

	if errorRate > 0.05 && retryEnabled != 1 {
		recs = append(recs, recommendation{
			ID:       "enable-retrypilot",
			Title:    "Enable Smart Retry",
			Reason:   fmt.Sprintf("Error rate is %.0f%% (%d/%d requests). RetryPilot can automatically retry failed requests with backoff.", errorRate*100, errorCount, totalRequests),
			Module:   "retrypilot",
			Priority: "high",
		})
	}

	writeJSON(w, map[string]any{
		"recommendations":         recs,
		"count":                   len(recs),
		"total_requests_analyzed": totalRequests,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// handleBreakerStates returns the current state of all circuit breakers.
// GET /api/proxy/breakers
func (a *App) handleBreakerStates(w http.ResponseWriter, r *http.Request) {
	if a.failover == nil {
		writeJSON(w, map[string]any{
			"enabled":  false,
			"breakers": map[string]any{},
			"message":  "failover module is not active",
		})
		return
	}
	states := a.failover.BreakerStates()
	writeJSON(w, map[string]any{
		"enabled":  true,
		"breakers": states,
		"count":    len(states),
	})
}

// handleResetAllBreakers resets all circuit breakers to closed state.
// POST /api/proxy/breakers/reset
func (a *App) handleResetAllBreakers(w http.ResponseWriter, r *http.Request) {
	if a.failover == nil {
		http.Error(w, `{"error":"failover module is not active"}`, http.StatusServiceUnavailable)
		return
	}
	a.failover.ResetAll()
	a.auditEvent("reset_all_breakers", "failover", nil)
	states := a.failover.BreakerStates()
	writeJSON(w, map[string]any{
		"reset":    true,
		"breakers": states,
	})
}

// handleResetBreaker resets a single provider's circuit breaker.
// POST /api/proxy/breakers/{name}/reset
func (a *App) handleResetBreaker(w http.ResponseWriter, r *http.Request) {
	if a.failover == nil {
		http.Error(w, `{"error":"failover module is not active"}`, http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"provider name required"}`, http.StatusBadRequest)
		return
	}
	if !a.failover.ResetBreaker(name) {
		http.Error(w, fmt.Sprintf(`{"error":"unknown provider: %s"}`, name), http.StatusNotFound)
		return
	}
	a.auditEvent("reset_breaker", "failover/"+name, map[string]string{"provider": name})
	states := a.failover.BreakerStates()
	writeJSON(w, map[string]any{
		"reset":    name,
		"breakers": states,
	})
}

// handleListAliases returns all configured model aliases.
// GET /api/proxy/aliases
func (a *App) handleListAliases(w http.ResponseWriter, r *http.Request) {
	if a.modelAlias == nil {
		writeJSON(w, map[string]any{"aliases": []any{}, "count": 0, "enabled": false})
		return
	}
	aliases := a.modelAlias.List()
	writeJSON(w, map[string]any{"aliases": aliases, "count": len(aliases), "enabled": true})
}

// handleSetAlias creates or updates a model alias.
// PUT /api/proxy/aliases  {"alias": "fast", "model": "gpt-4o-mini"}
func (a *App) handleSetAlias(w http.ResponseWriter, r *http.Request) {
	if a.modelAlias == nil {
		http.Error(w, `{"error":"model aliasing is not active"}`, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Alias string `json:"alias"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.Alias == "" || req.Model == "" {
		http.Error(w, `{"error":"alias and model are required"}`, http.StatusBadRequest)
		return
	}
	updated := a.modelAlias.Set(req.Alias, req.Model)
	// Persist to DB
	if a.conn != nil {
		_, _ = a.conn.Exec(`INSERT INTO proxy_aliases (alias, model) VALUES (?, ?)
			ON CONFLICT(alias) DO UPDATE SET model = excluded.model, updated_at = datetime('now')`,
			req.Alias, req.Model)
	}
	action := "created"
	if updated {
		action = "updated"
	}
	a.auditEvent("set_alias", "alias/"+req.Alias, map[string]string{"alias": req.Alias, "model": req.Model, "action": action})
	writeJSON(w, map[string]any{"alias": req.Alias, "model": req.Model, "action": action})
}

// handleDeleteAlias removes a model alias.
// DELETE /api/proxy/aliases/{alias}
func (a *App) handleDeleteAlias(w http.ResponseWriter, r *http.Request) {
	if a.modelAlias == nil {
		http.Error(w, `{"error":"model aliasing is not active"}`, http.StatusServiceUnavailable)
		return
	}
	alias := r.PathValue("alias")
	if alias == "" {
		http.Error(w, `{"error":"alias name required"}`, http.StatusBadRequest)
		return
	}
	if !a.modelAlias.Delete(alias) {
		http.Error(w, fmt.Sprintf(`{"error":"alias not found: %s"}`, alias), http.StatusNotFound)
		return
	}
	// Remove from DB
	if a.conn != nil {
		_, _ = a.conn.Exec(`DELETE FROM proxy_aliases WHERE alias = ?`, alias)
	}
	a.auditEvent("delete_alias", "alias/"+alias, map[string]string{"alias": alias})
	writeJSON(w, map[string]any{"deleted": alias})
}

// handleAliasStats returns alias resolution statistics.
// GET /api/proxy/aliases/stats
func (a *App) handleAliasStats(w http.ResponseWriter, r *http.Request) {
	if a.modelAlias == nil {
		writeJSON(w, map[string]any{"enabled": false})
		return
	}
	stats := a.modelAlias.Stats()
	stats["enabled"] = true
	writeJSON(w, stats)
}
