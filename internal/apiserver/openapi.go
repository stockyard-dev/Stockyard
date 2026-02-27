package apiserver

import (
	"encoding/json"
	"net/http"
)

// OpenAPISpec returns the OpenAPI 3.1 specification for the Stockyard API.
func OpenAPISpec() map[string]any {
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "Stockyard API",
			"version":     "1.0.0",
			"description": "LLM gateway with 6 apps, 54 middleware modules, and 16 providers. Drop-in OpenAI-compatible proxy with observability, trust policies, A/B testing, and workflow orchestration.",
			"contact":     map[string]string{"email": "hello@stockyard.dev", "url": "https://stockyard.dev"},
			"license":     map[string]string{"name": "MIT", "url": "https://github.com/stockyard-dev/stockyard/blob/main/LICENSE"},
		},
		"servers": []map[string]string{
			{"url": "http://localhost:4200", "description": "Local"},
			{"url": "https://stockyard-production.up.railway.app", "description": "Cloud"},
		},
		"tags": []map[string]string{
			{"name": "Proxy", "description": "OpenAI-compatible LLM proxy"},
			{"name": "Observe", "description": "Tracing, cost attribution, alerts"},
			{"name": "Trust", "description": "Policies, audit ledger, compliance"},
			{"name": "Studio", "description": "Experiments, benchmarks, templates"},
			{"name": "Forge", "description": "DAG workflow engine"},
			{"name": "Exchange", "description": "Config pack marketplace"},
			{"name": "Auth", "description": "Users, API keys, provider keys"},
			{"name": "System", "description": "Health, license, plans"},
		},
		"paths": openAPIPaths(),
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"AdminKey": map[string]any{
					"type": "apiKey", "in": "header", "name": "X-Admin-Key",
					"description": "Admin key for management API (set via STOCKYARD_ADMIN_KEY env var)",
				},
				"BearerAuth": map[string]any{
					"type": "http", "scheme": "bearer",
					"description": "User API key (sk-sy-...) or provider API key for auto-configure",
				},
			},
		},
	}
}

func openAPIPaths() map[string]any {
	ok := map[string]any{"description": "Success", "content": map[string]any{"application/json": map[string]any{"schema": map[string]string{"type": "object"}}}}
	err4 := map[string]any{"description": "Bad request"}
	err5 := map[string]any{"description": "Server error"}
	admin := []map[string]any{{"AdminKey": []string{}}}
	bearer := []map[string]any{{"BearerAuth": []string{}}}

	return map[string]any{
		// System
		"/health":      map[string]any{"get": ep("Health check", "System", nil, ok)},
		"/api/license":  map[string]any{"get": ep("License status and usage stats", "System", admin, ok)},
		"/api/plans":    map[string]any{"get": ep("List pricing plans", "System", nil, ok)},

		// Proxy
		"/v1/chat/completions": map[string]any{
			"post": epFull("OpenAI-compatible chat completions (proxied through middleware chain)", "Proxy", bearer, ok, err4, err5),
		},
		"/v1/embeddings": map[string]any{
			"post": epFull("OpenAI-compatible embeddings", "Proxy", bearer, ok, err4, err5),
		},
		"/api/proxy/modules": map[string]any{
			"get": ep("List all middleware modules with enabled state", "Proxy", nil, ok),
		},
		"/api/proxy/modules/{name}": map[string]any{
			"put": ep("Enable or disable a middleware module", "Proxy", nil, ok),
		},

		// Observe
		"/api/observe/overview":   map[string]any{"get": ep("Dashboard overview (traces, cost, alerts)", "Observe", admin, ok)},
		"/api/observe/traces":     map[string]any{"get": ep("List recent traces", "Observe", admin, ok)},
		"/api/observe/traces/{id}": map[string]any{"get": ep("Get trace detail with metadata", "Observe", admin, ok)},
		"/api/observe/timeseries": map[string]any{"get": ep("Time-bucketed metrics (period=24h|7d|30d)", "Observe", admin, ok)},
		"/api/observe/costs":      map[string]any{"get": ep("Cost breakdown by provider and model", "Observe", admin, ok)},
		"/api/observe/alerts":     map[string]any{
			"get":  ep("List alert rules", "Observe", admin, ok),
			"post": ep("Create alert rule", "Observe", admin, ok),
		},
		"/api/observe/alerts/{id}": map[string]any{
			"put":    ep("Update alert rule", "Observe", admin, ok),
			"delete": ep("Delete alert rule", "Observe", admin, ok),
		},

		// Trust
		"/api/trust/policies": map[string]any{
			"get":  ep("List trust policies", "Trust", admin, ok),
			"post": ep("Create trust policy (block/warn/redact/log)", "Trust", admin, ok),
		},
		"/api/trust/policies/{id}": map[string]any{
			"put":    ep("Update trust policy", "Trust", admin, ok),
			"delete": ep("Delete trust policy", "Trust", admin, ok),
		},
		"/api/trust/ledger": map[string]any{"get": ep("Audit ledger (hash-chained events)", "Trust", admin, ok)},

		// Studio
		"/api/studio/status":           map[string]any{"get": ep("Studio status (experiments, benchmarks, templates)", "Studio", admin, ok)},
		"/api/studio/experiments/run":   map[string]any{"post": ep("Run A/B experiment across models", "Studio", admin, ok)},
		"/api/studio/experiments/{id}":  map[string]any{"get": ep("Get experiment results", "Studio", admin, ok)},
		"/api/studio/benchmarks/run":    map[string]any{"post": ep("Run benchmark suite", "Studio", admin, ok)},
		"/api/studio/templates":         map[string]any{
			"get":  ep("List prompt templates", "Studio", admin, ok),
			"post": ep("Create prompt template", "Studio", admin, ok),
		},

		// Forge
		"/api/forge/workflows": map[string]any{
			"get":  ep("List DAG workflows", "Forge", admin, ok),
			"post": ep("Create workflow", "Forge", admin, ok),
		},
		"/api/forge/workflows/{id}": map[string]any{
			"get":    ep("Get workflow detail", "Forge", admin, ok),
			"delete": ep("Delete workflow", "Forge", admin, ok),
		},
		"/api/forge/workflows/{id}/run": map[string]any{
			"post": ep("Execute workflow", "Forge", admin, ok),
		},

		// Exchange
		"/api/exchange/status": map[string]any{"get": ep("Exchange status", "Exchange", admin, ok)},
		"/api/exchange/packs":  map[string]any{
			"get":  ep("List available packs", "Exchange", admin, ok),
			"post": ep("Create custom pack", "Exchange", admin, ok),
		},
		"/api/exchange/packs/{id}":         map[string]any{"get": ep("Get pack detail", "Exchange", admin, ok)},
		"/api/exchange/packs/{id}/install":  map[string]any{"post": ep("Install pack", "Exchange", admin, ok)},
		"/api/exchange/packs/{id}/uninstall": map[string]any{"post": ep("Uninstall pack", "Exchange", admin, ok)},

		// Auth
		"/api/auth/signup":                    map[string]any{"post": ep("Create user account + API key (public)", "Auth", nil, ok)},
		"/api/auth/users":                     map[string]any{"get": ep("List users", "Auth", admin, ok)},
		"/api/auth/users/{id}/keys":           map[string]any{
			"get":  ep("List user API keys", "Auth", admin, ok),
			"post": ep("Generate API key for user", "Auth", admin, ok),
		},
		"/api/auth/users/{id}/providers/{provider}": map[string]any{
			"put":    ep("Set provider API key for user", "Auth", admin, ok),
			"delete": ep("Delete provider key for user", "Auth", admin, ok),
		},
		"/api/auth/me":                        map[string]any{"get": ep("Current user info (via API key)", "Auth", bearer, ok)},
		"/api/auth/me/keys":                   map[string]any{"get": ep("List my API keys", "Auth", bearer, ok)},
		"/api/auth/me/providers/{provider}":   map[string]any{
			"put":    ep("Set my provider key", "Auth", bearer, ok),
			"get":    ep("List my provider keys", "Auth", bearer, ok),
			"delete": ep("Delete my provider key", "Auth", bearer, ok),
		},
		"/api/auth/me/usage": map[string]any{"get": ep("My usage stats (requests, cost, tokens)", "Auth", bearer, ok)},

		// Checkout
		"/api/checkout": map[string]any{"post": ep("Create Stripe checkout session", "System", nil, ok)},
	}
}

func ep(desc, tag string, security []map[string]any, resp map[string]any) map[string]any {
	e := map[string]any{
		"summary":   desc,
		"tags":      []string{tag},
		"responses": map[string]any{"200": resp},
	}
	if security != nil {
		e["security"] = security
	}
	return e
}

func epFull(desc, tag string, security []map[string]any, ok, err4, err5 map[string]any) map[string]any {
	e := map[string]any{
		"summary": desc,
		"tags":    []string{tag},
		"responses": map[string]any{
			"200": ok,
			"400": err4,
			"500": err5,
		},
	}
	if security != nil {
		e["security"] = security
	}
	return e
}

// HandleOpenAPI serves the OpenAPI spec as JSON.
func HandleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(OpenAPISpec())
}
