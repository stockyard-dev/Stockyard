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
			"description": "LLM gateway with 16 apps, 66 middleware modules, and 16 providers. Drop-in OpenAI-compatible proxy with observability, trust policies, A/B testing, and workflow orchestration.",
			"contact":     map[string]string{"email": "hello@stockyard.dev", "url": "https://stockyard.dev"},
			"license":     map[string]string{"name": "MIT", "url": "https://github.com/stockyard-dev/stockyard/blob/main/LICENSE"},
		},
		"servers": []map[string]string{
			{"url": "http://localhost:4200", "description": "Local"},
			{"url": "https://stockyard.dev", "description": "Cloud"},
		},
		"tags": []map[string]string{
			{"name": "Chute", "description": "OpenAI-compatible LLM proxy (formerly Proxy)"},
			{"name": "Lookout", "description": "Tracing, cost attribution, alerts (formerly Observe)"},
			{"name": "Brand", "description": "Policies, audit ledger, compliance (formerly Trust)"},
			{"name": "Tack Room", "description": "Experiments, benchmarks, templates (formerly Studio)"},
			{"name": "Forge", "description": "DAG workflow engine"},
			{"name": "Trading Post", "description": "Config pack marketplace (formerly Exchange)"},
			{"name": "Auth", "description": "Users, API keys, provider keys"},
			{"name": "Playground", "description": "Shareable playground sessions"},
			{"name": "Webhooks", "description": "Event delivery to external endpoints"},
			{"name": "Config", "description": "Configuration export, import, and diff"},
			{"name": "System", "description": "Health, license, plans"},
			{"name": "Lasso", "description": "Request replay and model comparison"},
			{"name": "Drover", "description": "Autopilot routing and model calibration"},
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
		"/api/license": map[string]any{"get": ep("License status and usage stats", "System", admin, ok)},
		"/api/plans":   map[string]any{"get": ep("List pricing plans", "System", nil, ok)},

		// Proxy
		"/v1/chat/completions": map[string]any{
			"post": epFull("OpenAI-compatible chat completions (proxied through middleware chain)", "Chute", bearer, ok, err4, err5),
		},
		"/v1/embeddings": map[string]any{
			"post": epFull("OpenAI-compatible embeddings", "Chute", bearer, ok, err4, err5),
		},
		"/api/proxy/modules": map[string]any{
			"get": ep("List all middleware modules with enabled state", "Chute", nil, ok),
		},
		"/api/proxy/modules/{name}": map[string]any{
			"put": ep("Enable or disable a middleware module", "Chute", nil, ok),
		},

		// Observe
		"/api/observe/overview":    map[string]any{"get": ep("Dashboard overview (traces, cost, alerts)", "Lookout", admin, ok)},
		"/api/observe/traces":      map[string]any{"get": ep("List recent traces", "Lookout", admin, ok)},
		"/api/observe/traces/{id}": map[string]any{"get": ep("Get trace detail with metadata", "Lookout", admin, ok)},
		"/api/observe/timeseries":  map[string]any{"get": ep("Time-bucketed metrics (period=24h|7d|30d)", "Lookout", admin, ok)},
		"/api/observe/costs":       map[string]any{"get": ep("Cost breakdown by provider and model", "Lookout", admin, ok)},
		"/api/observe/alerts": map[string]any{
			"get":  ep("List alert rules", "Lookout", admin, ok),
			"post": ep("Create alert rule", "Lookout", admin, ok),
		},
		"/api/observe/alerts/{id}": map[string]any{
			"put":    ep("Update alert rule", "Lookout", admin, ok),
			"delete": ep("Delete alert rule", "Lookout", admin, ok),
		},

		// Trust
		"/api/trust/policies": map[string]any{
			"get":  ep("List trust policies", "Brand", admin, ok),
			"post": ep("Create trust policy (block/warn/redact/log)", "Brand", admin, ok),
		},
		"/api/trust/policies/{id}": map[string]any{
			"put":    ep("Update trust policy", "Brand", admin, ok),
			"delete": ep("Delete trust policy", "Brand", admin, ok),
		},
		"/api/trust/ledger": map[string]any{"get": ep("Audit ledger (hash-chained events)", "Brand", admin, ok)},

		// Studio
		"/api/studio/status":           map[string]any{"get": ep("Studio status (experiments, benchmarks, templates)", "Tack Room", admin, ok)},
		"/api/studio/experiments/run":  map[string]any{"post": ep("Run A/B experiment across models", "Tack Room", admin, ok)},
		"/api/studio/experiments/{id}": map[string]any{"get": ep("Get experiment results", "Tack Room", admin, ok)},
		"/api/studio/benchmarks/run":   map[string]any{"post": ep("Run benchmark suite", "Tack Room", admin, ok)},
		"/api/studio/templates": map[string]any{
			"get":  ep("List prompt templates", "Tack Room", admin, ok),
			"post": ep("Create prompt template", "Tack Room", admin, ok),
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
		"/api/exchange/status": map[string]any{"get": ep("Exchange status", "Trading Post", admin, ok)},
		"/api/exchange/packs": map[string]any{
			"get":  ep("List available packs", "Trading Post", admin, ok),
			"post": ep("Create custom pack", "Trading Post", admin, ok),
		},
		"/api/exchange/packs/{id}":           map[string]any{"get": ep("Get pack detail", "Trading Post", admin, ok)},
		"/api/exchange/packs/{id}/install":   map[string]any{"post": ep("Install pack", "Trading Post", admin, ok)},
		"/api/exchange/packs/{id}/uninstall": map[string]any{"post": ep("Uninstall pack", "Trading Post", admin, ok)},

		// Auth
		"/api/auth/signup": map[string]any{"post": ep("Create user account + API key (public)", "Auth", nil, ok)},
		"/api/auth/users":  map[string]any{"get": ep("List users", "Auth", admin, ok)},
		"/api/auth/users/{id}/keys": map[string]any{
			"get":  ep("List user API keys", "Auth", admin, ok),
			"post": ep("Generate API key for user", "Auth", admin, ok),
		},
		"/api/auth/users/{id}/keys/{keyId}/rotate": map[string]any{
			"post": ep("Rotate API key (revoke old, generate new)", "Auth", admin, ok),
		},
		"/api/auth/users/{id}/providers/{provider}": map[string]any{
			"put":    ep("Set provider API key for user", "Auth", admin, ok),
			"delete": ep("Delete provider key for user", "Auth", admin, ok),
		},
		"/api/auth/me":      map[string]any{"get": ep("Current user info (via API key)", "Auth", bearer, ok)},
		"/api/auth/me/keys": map[string]any{"get": ep("List my API keys", "Auth", bearer, ok)},
		"/api/auth/me/providers/{provider}": map[string]any{
			"put":    ep("Set my provider key", "Auth", bearer, ok),
			"get":    ep("List my provider keys", "Auth", bearer, ok),
			"delete": ep("Delete my provider key", "Auth", bearer, ok),
		},
		"/api/auth/me/usage": map[string]any{"get": ep("My usage stats (requests, cost, tokens)", "Auth", bearer, ok)},

		// Checkout
		"/api/checkout": map[string]any{"post": ep("Create Stripe checkout session", "System", nil, ok)},

		// Playground
		"/api/playground/share":      map[string]any{"post": ep("Create shareable playground session (30-day TTL)", "Playground", nil, ok)},
		"/api/playground/share/{id}": map[string]any{"get": ep("Retrieve shared playground session", "Playground", nil, ok)},

		// Webhooks
		"/api/webhooks": map[string]any{
			"get":  ep("List registered webhooks", "Webhooks", admin, ok),
			"post": ep("Register a webhook endpoint", "Webhooks", admin, ok),
		},
		"/api/webhooks/{id}": map[string]any{"delete": ep("Delete a webhook", "Webhooks", admin, ok)},
		"/api/webhooks/test": map[string]any{"post": ep("Send test event to all webhooks", "Webhooks", admin, ok)},

		// Status
		"/api/status": map[string]any{"get": ep("System status, uptime, component health, request metrics", "System", nil, ok)},

		// Config
		"/api/config/export": map[string]any{"get": ep("Export full config snapshot (modules, webhooks, policies)", "Config", admin, ok)},
		"/api/config/import": map[string]any{"post": ep("Import config snapshot — apply module states, add webhooks/policies", "Config", admin, ok)},
		"/api/config/diff":   map[string]any{"post": ep("Diff current config against uploaded snapshot", "Config", admin, ok)},
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
	origin := r.Header.Get("Origin")
	if corsAllowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	json.NewEncoder(w).Encode(OpenAPISpec())
}
