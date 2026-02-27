package engine

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/stockyard-dev/stockyard/internal/slog"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// appHooksMiddleware wraps every proxy request and writes:
//   - A trace row into observe_traces + cost rollup into observe_cost_daily
//   - An audit event into trust_ledger (append-only hash chain)
//
// This is the outermost middleware — it sees every request and response.
func appHooksMiddleware(conn *sql.DB) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()
			traceID := genTraceID()

			// Call the rest of the chain
			resp, err := next(ctx, req)

			duration := time.Since(start)

			// Structured request log
			status := "ok"
			if err != nil {
				status = "error"
			} else if resp != nil && resp.CacheHit {
				status = "cache_hit"
			}
			logFields := []any{
				"trace_id", traceID,
				"provider", req.Provider,
				"model", req.Model,
				"status", status,
				"duration_ms", duration.Milliseconds(),
			}
			if resp != nil {
				logFields = append(logFields,
					"tokens_in", resp.Usage.PromptTokens,
					"tokens_out", resp.Usage.CompletionTokens,
				)
			}
			if err != nil {
				logFields = append(logFields, "error", err)
				slog.Error("proxy request failed", logFields...)
			} else {
				slog.Info("proxy request", logFields...)
			}

			go recordObserveTrace(conn, traceID, req, resp, err, duration)
			go recordTrustEvent(conn, traceID, req, resp, err, duration)

			return resp, err
		}
	}
}

// recordObserveTrace writes a trace + daily cost rollup to Observe tables.
func recordObserveTrace(conn *sql.DB, traceID string, req *provider.Request, resp *provider.Response, reqErr error, dur time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[observe-hook] panic: %v", r)
		}
	}()

	prov := req.Provider
	model := req.Model
	status := "ok"
	var tokIn, tokOut int64
	var costUSD float64

	if reqErr != nil {
		status = "error"
	}
	if resp != nil {
		if resp.Provider != "" {
			prov = resp.Provider
		}
		if resp.Model != "" {
			model = resp.Model
		}
		tokIn = int64(resp.Usage.PromptTokens)
		tokOut = int64(resp.Usage.CompletionTokens)
		if resp.CacheHit {
			status = "cache_hit"
		}
		// Rough cost estimate: $0.002 per 1K input tokens, $0.006 per 1K output tokens
		costUSD = float64(tokIn)/1000*0.002 + float64(tokOut)/1000*0.006
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := conn.Exec(`INSERT INTO observe_traces 
		(id, request_id, service, operation, provider, model, status, duration_ms, tokens_in, tokens_out, cost_usd, metadata_json, created_at) 
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		traceID, traceID, "proxy", "chat.completion", prov, model, status,
		dur.Milliseconds(), tokIn, tokOut, costUSD, "{}", now)
	if err != nil {
		// Table might not exist if apps aren't registered — silent skip
		return
	}

	// Cost rollup
	today := time.Now().UTC().Format("2006-01-02")
	conn.Exec(`INSERT INTO observe_cost_daily (date, provider, model, requests, tokens_in, tokens_out, cost_usd) 
		VALUES (?,?,?,1,?,?,?) 
		ON CONFLICT(date, provider, model) DO UPDATE SET 
			requests=requests+1, tokens_in=tokens_in+excluded.tokens_in, 
			tokens_out=tokens_out+excluded.tokens_out, cost_usd=cost_usd+excluded.cost_usd`,
		today, prov, model, tokIn, tokOut, costUSD)
}

// recordTrustEvent appends to the immutable audit ledger.
func recordTrustEvent(conn *sql.DB, traceID string, req *provider.Request, resp *provider.Response, reqErr error, dur time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[trust-hook] panic: %v", r)
		}
	}()

	action := "proxy.request"
	resource := req.Model
	actor := req.UserID
	if actor == "" {
		actor = req.ClientIP
	}

	status := "ok"
	if reqErr != nil {
		status = "error"
		action = "proxy.error"
	}

	detail := fmt.Sprintf(`{"trace_id":"%s","provider":"%s","model":"%s","status":"%s","duration_ms":%d}`,
		traceID, req.Provider, req.Model, status, dur.Milliseconds())

	// Get previous hash for chain
	var prevHash string
	conn.QueryRow("SELECT hash FROM trust_ledger ORDER BY id DESC LIMIT 1").Scan(&prevHash)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prevHash, "proxy.request", action, resource, detail, now)
	h := sha256.Sum256([]byte(hashInput))
	hash := hex.EncodeToString(h[:])

	_, err := conn.Exec(`INSERT INTO trust_ledger 
		(event_type, actor, resource, action, detail_json, prev_hash, hash, created_at) 
		VALUES (?,?,?,?,?,?,?,?)`,
		"proxy.request", actor, resource, action, detail, prevHash, hash, now)
	if err != nil {
		// Table might not exist — silent skip
		return
	}
}

// seedProxyModules populates the proxy_modules table from active feature flags
// so the /api/proxy/modules endpoint returns real data.
func seedProxyModules(conn *sql.DB, pc ProductConfig) {
	var count int
	conn.QueryRow("SELECT COUNT(*) FROM proxy_modules").Scan(&count)
	if count > 0 {
		return // Already seeded
	}

	type mod struct {
		name     string
		category string
		enabled  bool
		priority int
	}

	modules := []mod{
		// Routing
		{"fallbackrouter", "routing", pc.Features.Failover, 10},
		{"modelswitch", "routing", pc.Features.ModelSwitch, 11},
		{"regionroute", "routing", pc.Features.RegionRoute, 12},
		{"localsync", "routing", pc.Features.LocalSync, 13},
		{"abrouter", "routing", pc.Features.ABRouter, 14},
		// Caching
		{"cachelayer", "caching", pc.Features.Cache, 20},
		{"embedcache", "caching", pc.Features.EmbedCache, 21},
		{"semanticcache", "caching", pc.Features.SemanticCache, 22},
		// Cost
		{"costcap", "cost", pc.Features.SpendCaps, 30},
		{"tierdrop", "cost", pc.Features.TierDrop, 31},
		{"idlekill", "cost", pc.Features.IdleKill, 32},
		{"outputcap", "cost", pc.Features.OutputCap, 33},
		{"usagepulse", "cost", pc.Features.UsagePulse, 34},
		// Rate
		{"rateshield", "rate", pc.Features.RateLimiting, 40},
		// Keys
		{"keypool", "keys", pc.Features.KeyPool, 50},
		// Transform
		{"promptslim", "transform", pc.Features.PromptSlim, 60},
		{"tokentrim", "transform", pc.Features.TokenTrim, 61},
		{"contextpack", "transform", pc.Features.ContextPack, 62},
		{"chatmem", "transform", pc.Features.ChatMem, 63},
		{"langbridge", "transform", pc.Features.LangBridge, 64},
		{"voicebridge", "transform", pc.Features.VoiceBridge, 65},
		// Validate
		{"structuredshield", "validate", pc.Features.Validation, 70},
		{"evalgate", "validate", pc.Features.EvalGate, 71},
		{"codefence", "validate", pc.Features.CodeFence, 72},
		// Safety
		{"promptguard", "safety", pc.Features.PromptGuard, 80},
		{"toxicfilter", "safety", pc.Features.ToxicFilter, 81},
		{"guardrail", "safety", pc.Features.GuardRail, 82},
		{"agegate", "safety", pc.Features.AgeGate, 83},
		{"hallucicheck", "safety", pc.Features.HalluciCheck, 84},
		{"secretscan", "safety", pc.Features.SecretScan, 85},
		{"agentguard", "safety", pc.Features.AgentGuard, 86},
		// Shims
		{"anthrofit", "shims", pc.Features.AnthroFit, 90},
		{"geminishim", "shims", pc.Features.GeminiShim, 91},
		// Stream
		{"streamsnap", "stream", pc.Features.StreamSnap, 100},
		// Multimodal
		{"imageproxy", "multimodal", pc.Features.ImageProxy, 110},
		// Tenant
		{"tenantwall", "tenant", pc.Features.TenantWall, 120},
		{"ipfence", "tenant", pc.Features.IPFence, 121},
		// Observability
		{"llmtap", "observe", pc.Features.LLMTap, 130},
		{"tracelink", "observe", pc.Features.TraceLink, 131},
		{"alertpulse", "observe", pc.Features.AlertPulse, 132},
		{"driftwatch", "observe", pc.Features.DriftWatch, 133},
		// Trust
		{"compliancelog", "trust", pc.Features.ComplianceLog, 140},
		{"feedbackloop", "trust", pc.Features.FeedbackLoop, 141},
		// Studio
		{"promptpad", "studio", pc.Features.PromptPad, 150},
		{"promptlint", "studio", pc.Features.PromptLint, 151},
		{"approvalgate", "studio", pc.Features.ApprovalGate, 152},
		// Forge
		{"batchqueue", "forge", pc.Features.BatchQueue, 160},
		{"multicall", "forge", pc.Features.MultiCall, 161},
		{"mockllm", "forge", pc.Features.MockLLM, 162},
		// Exchange
		{"devproxy", "exchange", pc.Features.DevProxy, 170},
	}

	for _, m := range modules {
		enabled := 0
		if m.enabled {
			enabled = 1
		}
		conn.Exec("INSERT OR IGNORE INTO proxy_modules (name, category, enabled, priority) VALUES (?,?,?,?)",
			m.name, m.category, enabled, m.priority)
	}

	log.Printf("[proxy] seeded %d modules into proxy_modules table", len(modules))
}

// seedProxyProviders populates the proxy_providers table from configured providers.
func seedProxyProviders(conn *sql.DB, providers map[string]provider.Provider) {
	var count int
	conn.QueryRow("SELECT COUNT(*) FROM proxy_providers").Scan(&count)
	if count > 0 {
		return
	}

	for name := range providers {
		conn.Exec("INSERT OR IGNORE INTO proxy_providers (name, status) VALUES (?, 'active')", name)
	}

	if len(providers) > 0 {
		log.Printf("[proxy] seeded %d providers into proxy_providers table", len(providers))
	}
}

func genTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "tr_" + hex.EncodeToString(b)
}

// seedExchangePacks creates starter packs in the exchange marketplace.
func seedExchangePacks(conn *sql.DB) {
	var count int
	conn.QueryRow("SELECT COUNT(*) FROM exchange_packs").Scan(&count)
	if count > 0 {
		return
	}

	type pack struct {
		slug, name, desc, author, tags string
		content                        string
	}

	packs := []pack{
		{
			slug: "safety-essentials", name: "Safety Essentials", author: "Stockyard",
			desc: "Core safety modules: PII redaction, content filtering, prompt injection detection, and trust policies",
			tags: `["safety","trust","recommended"]`,
			content: `{
				"modules": [
					{"name": "pii_redactor", "enabled": true},
					{"name": "content_filter", "enabled": true},
					{"name": "prompt_injection", "enabled": true},
					{"name": "toxicity_filter", "enabled": true}
				],
				"policies": [
					{"name": "block-pii-leak", "policy_type": "block", "pattern": "ssn|credit.card|password", "description": "Block responses containing PII patterns", "enabled": true},
					{"name": "warn-high-toxicity", "policy_type": "warn", "pattern": "toxicity_score > 0.8", "description": "Flag high-toxicity completions", "enabled": true}
				],
				"alerts": [
					{"name": "pii-leak-alert", "metric": "pii_detections", "condition": "gt", "threshold": 0, "window": "5m", "channel": "default", "enabled": true},
					{"name": "injection-spike", "metric": "injection_blocks", "condition": "gt", "threshold": 5, "window": "1h", "channel": "default", "enabled": true}
				]
			}`,
		},
		{
			slug: "cost-control", name: "Cost Control Pack", author: "Stockyard",
			desc: "Rate limiting, cost caps, token budgets, and cost alerting — keep your LLM spend under control",
			tags: `["cost","billing","recommended"]`,
			content: `{
				"modules": [
					{"name": "rateshield", "enabled": true},
					{"name": "cost_cap", "enabled": true},
					{"name": "token_budget", "enabled": true},
					{"name": "cache", "enabled": true}
				],
				"alerts": [
					{"name": "daily-cost-limit", "metric": "cost", "condition": "gt", "threshold": 50.0, "window": "24h", "channel": "default", "enabled": true},
					{"name": "high-token-request", "metric": "tokens", "condition": "gt", "threshold": 100000, "window": "5m", "channel": "default", "enabled": true},
					{"name": "rate-limit-spike", "metric": "rate_limited", "condition": "gt", "threshold": 20, "window": "5m", "channel": "default", "enabled": true}
				]
			}`,
		},
		{
			slug: "openai-quickstart", name: "OpenAI Quickstart", author: "Stockyard",
			desc: "Pre-configured OpenAI provider with model routing, caching, and a starter prompt template",
			tags: `["openai","quickstart","provider"]`,
			content: `{
				"providers": [
					{"name": "openai", "base_url": "https://api.openai.com/v1", "auth_type": "bearer", "priority": 1, "models": "gpt-4o,gpt-4o-mini,gpt-4-turbo,o1,o1-mini,o3-mini"}
				],
				"routes": [
					{"pattern": "gpt-*", "provider": "openai", "priority": 1, "enabled": true},
					{"pattern": "o1*", "provider": "openai", "priority": 1, "enabled": true},
					{"pattern": "o3*", "provider": "openai", "priority": 1, "enabled": true}
				],
				"modules": [
					{"name": "cache", "enabled": true},
					{"name": "retry", "enabled": true}
				],
				"templates": [
					{"slug": "general-assistant", "name": "General Assistant", "description": "Versatile assistant template with system instructions", "template": "You are a helpful, accurate, and concise assistant. Answer the user's question directly.\n\nUser: {{input}}", "model": "gpt-4o-mini", "tags": "general,starter"}
				]
			}`,
		},
		{
			slug: "anthropic-quickstart", name: "Anthropic Quickstart", author: "Stockyard",
			desc: "Pre-configured Anthropic provider with Claude model routing and prompt template",
			tags: `["anthropic","quickstart","provider"]`,
			content: `{
				"providers": [
					{"name": "anthropic", "base_url": "https://api.anthropic.com/v1", "auth_type": "header", "priority": 1, "models": "claude-sonnet-4-20250514,claude-haiku-4-5-20251001,claude-opus-4-5-20250918"}
				],
				"routes": [
					{"pattern": "claude-*", "provider": "anthropic", "priority": 1, "enabled": true}
				],
				"modules": [
					{"name": "cache", "enabled": true},
					{"name": "retry", "enabled": true}
				],
				"templates": [
					{"slug": "claude-analyst", "name": "Claude Analyst", "description": "Analysis-focused template for Claude", "template": "Analyze the following and provide structured insights:\n\n{{input}}", "model": "claude-sonnet-4-20250514", "tags": "analysis,claude"}
				]
			}`,
		},
		{
			slug: "multi-provider-failover", name: "Multi-Provider Failover", author: "Stockyard",
			desc: "Configure OpenAI + Anthropic with automatic failover, health checks, and load balancing",
			tags: `["failover","reliability","advanced"]`,
			content: `{
				"providers": [
					{"name": "openai", "base_url": "https://api.openai.com/v1", "auth_type": "bearer", "priority": 1, "models": "gpt-4o,gpt-4o-mini"},
					{"name": "anthropic", "base_url": "https://api.anthropic.com/v1", "auth_type": "header", "priority": 2, "models": "claude-sonnet-4-20250514,claude-haiku-4-5-20251001"}
				],
				"modules": [
					{"name": "failover", "enabled": true},
					{"name": "healthcheck", "enabled": true},
					{"name": "loadbalance", "enabled": true},
					{"name": "retry", "enabled": true},
					{"name": "circuit_breaker", "enabled": true}
				],
				"alerts": [
					{"name": "provider-down", "metric": "error_rate", "condition": "gt", "threshold": 0.5, "window": "5m", "channel": "default", "enabled": true},
					{"name": "failover-triggered", "metric": "failovers", "condition": "gt", "threshold": 0, "window": "5m", "channel": "default", "enabled": true}
				]
			}`,
		},
		{
			slug: "eval-suite", name: "Evaluation Suite", author: "Stockyard",
			desc: "Workflows and templates for systematic LLM evaluation: accuracy, hallucination detection, and regression testing",
			tags: `["eval","testing","studio"]`,
			content: `{
				"workflows": [
					{
						"slug": "eval-accuracy", "name": "Accuracy Evaluator",
						"description": "Run a prompt through an LLM then grade the response for accuracy",
						"steps": [
							{"id": "generate", "type": "llm", "config": {"model": "gpt-4o-mini", "prompt": "{{input}}"}},
							{"id": "grade", "type": "llm", "depends_on": ["generate"], "config": {"model": "gpt-4o", "system": "You are an evaluator. Grade the response for accuracy on a scale of 1-10. Respond with JSON: {\"score\": N, \"reasoning\": \"...\"}", "prompt": "Original question: {{input}}\n\nResponse to grade:\n{{steps.generate.output}}"}},
							{"id": "result", "type": "transform", "depends_on": ["grade"], "config": {"expression": "extract_json"}}
						]
					},
					{
						"slug": "hallucination-check", "name": "Hallucination Detector",
						"description": "Generate a response then check for hallucinated claims",
						"steps": [
							{"id": "respond", "type": "llm", "config": {"model": "gpt-4o-mini", "prompt": "{{input}}"}},
							{"id": "check", "type": "llm", "depends_on": ["respond"], "config": {"model": "gpt-4o", "system": "Identify any claims in the response that appear fabricated, unverifiable, or inconsistent. List each with confidence level.", "prompt": "Response to check:\n{{steps.respond.output}}"}}
						]
					}
				],
				"templates": [
					{"slug": "eval-rubric", "name": "Eval Rubric", "description": "Rubric-based evaluation template", "template": "Evaluate the following response using this rubric:\n- Accuracy (1-10)\n- Completeness (1-10)\n- Clarity (1-10)\n- Relevance (1-10)\n\nResponse:\n{{input}}\n\nProvide scores as JSON.", "model": "gpt-4o", "tags": "eval,grading"}
				]
			}`,
		},
	}

	for _, p := range packs {
		id := "pk_" + p.slug
		conn.Exec(`INSERT OR IGNORE INTO exchange_packs (id, slug, name, description, author, pack_type, tags_json) VALUES (?,?,?,?,?,?,?)`,
			id, p.slug, p.name, p.desc, p.author, "config", p.tags)
		conn.Exec(`INSERT OR IGNORE INTO exchange_pack_versions (pack_id, version, content_json) VALUES (?,?,?)`,
			id, "1.0.0", p.content)
	}

	// Wave 2 packs (OR IGNORE so they appear on existing installs too)
	extraPacks := []pack{
		{
			slug: "deepseek-starter", name: "DeepSeek Starter", author: "Stockyard",
			desc: "Pre-configured DeepSeek provider with cost-optimized model routing and caching",
			tags: `["deepseek","quickstart","provider","cost"]`,
			content: `{
				"providers": [{"name": "deepseek", "base_url": "https://api.deepseek.com/v1", "auth_type": "bearer", "priority": 1, "models": "deepseek-chat,deepseek-coder,deepseek-reasoner"}],
				"routes": [{"pattern": "deepseek-*", "provider": "deepseek", "priority": 1, "enabled": true}],
				"modules": [{"name": "cache", "enabled": true}, {"name": "retry", "enabled": true}, {"name": "cost_cap", "enabled": true}]
			}`,
		},
		{
			slug: "groq-speed", name: "Groq Speed Pack", author: "Stockyard",
			desc: "Groq provider optimized for ultra-low latency inference with Llama and Mixtral models",
			tags: `["groq","quickstart","provider","speed"]`,
			content: `{
				"providers": [{"name": "groq", "base_url": "https://api.groq.com/openai/v1", "auth_type": "bearer", "priority": 1, "models": "llama-3.3-70b-versatile,llama-3.1-8b-instant,mixtral-8x7b-32768"}],
				"routes": [{"pattern": "llama-*", "provider": "groq", "priority": 1, "enabled": true}, {"pattern": "mixtral-*", "provider": "groq", "priority": 1, "enabled": true}],
				"modules": [{"name": "cache", "enabled": true}],
				"alerts": [{"name": "groq-latency", "metric": "latency_p95", "condition": "gt", "threshold": 500, "window": "5m", "channel": "default", "enabled": true}]
			}`,
		},
		{
			slug: "mistral-eu", name: "Mistral EU Pack", author: "Stockyard",
			desc: "Mistral AI provider for EU-hosted inference with GDPR-compliant routing",
			tags: `["mistral","quickstart","provider","eu","compliance"]`,
			content: `{
				"providers": [{"name": "mistral", "base_url": "https://api.mistral.ai/v1", "auth_type": "bearer", "priority": 1, "models": "mistral-large-latest,mistral-small-latest,codestral-latest"}],
				"routes": [{"pattern": "mistral-*", "provider": "mistral", "priority": 1, "enabled": true}, {"pattern": "codestral-*", "provider": "mistral", "priority": 1, "enabled": true}],
				"modules": [{"name": "cache", "enabled": true}, {"name": "retry", "enabled": true}],
				"policies": [{"name": "eu-data-residency", "policy_type": "log", "pattern": ".*", "description": "Log all requests for GDPR compliance audit trail", "enabled": true}]
			}`,
		},
		{
			slug: "local-llm", name: "Local LLM Pack", author: "Stockyard",
			desc: "Run models locally via Ollama or LM Studio — zero API costs, full privacy",
			tags: `["ollama","lm-studio","local","privacy"]`,
			content: `{
				"providers": [
					{"name": "ollama", "base_url": "http://localhost:11434/v1", "auth_type": "none", "priority": 1, "models": "llama3,codellama,mistral,phi3"},
					{"name": "lm-studio", "base_url": "http://localhost:1234/v1", "auth_type": "none", "priority": 2, "models": "local-model"}
				],
				"routes": [{"pattern": "llama*", "provider": "ollama", "priority": 1, "enabled": true}],
				"modules": [{"name": "retry", "enabled": true}, {"name": "healthcheck", "enabled": true}]
			}`,
		},
		{
			slug: "compliance-hipaa", name: "HIPAA Compliance Pack", author: "Stockyard",
			desc: "Trust policies and audit logging configured for HIPAA-adjacent compliance in healthcare contexts",
			tags: `["compliance","hipaa","healthcare","trust"]`,
			content: `{
				"modules": [{"name": "pii_redactor", "enabled": true}, {"name": "content_filter", "enabled": true}, {"name": "audit_log", "enabled": true}],
				"policies": [
					{"name": "redact-phi", "policy_type": "redact", "pattern": "SSN|DOB|date of birth|medical record|patient id", "description": "Redact protected health information patterns", "enabled": true},
					{"name": "block-phi-storage", "policy_type": "block", "pattern": "store.*patient|save.*health.*record|persist.*PHI", "description": "Block requests that attempt to store PHI", "enabled": true},
					{"name": "audit-all", "policy_type": "log", "pattern": ".*", "description": "Log all LLM interactions for compliance audit trail", "enabled": true}
				],
				"alerts": [{"name": "phi-redaction", "metric": "pii_detections", "condition": "gt", "threshold": 0, "window": "5m", "channel": "default", "enabled": true}]
			}`,
		},
		{
			slug: "dev-productivity", name: "Developer Productivity", author: "Stockyard",
			desc: "Coding-focused configuration with code models, prompt templates for development tasks, and cost tracking",
			tags: `["developer","coding","productivity"]`,
			content: `{
				"modules": [{"name": "cache", "enabled": true}, {"name": "retry", "enabled": true}, {"name": "cost_cap", "enabled": true}],
				"templates": [
					{"slug": "code-review", "name": "Code Review", "description": "Automated code review with actionable feedback", "template": "Review this code for bugs, security issues, and improvements. Be specific.\n\n{{input}}", "model": "gpt-4o", "tags": "code,review"},
					{"slug": "explain-code", "name": "Explain Code", "description": "Clear code explanation for onboarding", "template": "Explain this code clearly: what it does, how it works, notable patterns.\n\n{{input}}", "model": "gpt-4o-mini", "tags": "code,explain"},
					{"slug": "write-tests", "name": "Write Tests", "description": "Generate unit tests with edge cases", "template": "Write comprehensive unit tests for this code. Include edge cases.\n\n{{input}}", "model": "gpt-4o", "tags": "code,testing"},
					{"slug": "commit-msg", "name": "Commit Message", "description": "Generate conventional commit message", "template": "Write a conventional commit message for this diff.\n\n{{input}}", "model": "gpt-4o-mini", "tags": "code,git"}
				]
			}`,
		},
	}
	for _, p := range extraPacks {
		id := "pk_" + p.slug
		conn.Exec(`INSERT OR IGNORE INTO exchange_packs (id, slug, name, description, author, pack_type, tags_json) VALUES (?,?,?,?,?,?,?)`,
			id, p.slug, p.name, p.desc, p.author, "config", p.tags)
		conn.Exec(`INSERT OR IGNORE INTO exchange_pack_versions (pack_id, version, content_json) VALUES (?,?,?)`,
			id, "1.0.0", p.content)
	}

	log.Printf("[exchange] seeded %d starter packs + %d extra packs", len(packs), len(extraPacks))
}

// seedForgeData populates the Forge tool registry and additional demo workflows.
func seedForgeData(conn *sql.DB) {
	// ── Seed tools ──────────────────────────────────────────────────
	type tool struct {
		name, desc, ttype, handler string
		schema                     string
	}
	tools := []tool{
		{
			name: "echo", desc: "Returns input unchanged (useful for testing DAGs)", ttype: "builtin", handler: "echo",
			schema: `{"input": {"type": "string", "description": "Any input to echo back"}}`,
		},
		{
			name: "json_validate", desc: "Validates that input is well-formed JSON", ttype: "builtin", handler: "json_validate",
			schema: `{"input": {"type": "string", "description": "JSON string to validate"}}`,
		},
		{
			name: "timestamp", desc: "Returns the current ISO 8601 timestamp", ttype: "builtin", handler: "timestamp",
			schema: `{}`,
		},
		{
			name: "word_count", desc: "Counts words in the input text", ttype: "builtin", handler: "word_count",
			schema: `{"input": {"type": "string", "description": "Text to count words in"}}`,
		},
		{
			name: "summarize_results", desc: "Aggregates all previous step outputs into a JSON summary", ttype: "builtin", handler: "summarize_results",
			schema: `{}`,
		},
	}
	for _, t := range tools {
		conn.Exec(`INSERT OR IGNORE INTO forge_tools (name, description, type, schema_json, handler, enabled) VALUES (?,?,?,?,?,1)`,
			t.name, t.desc, t.ttype, t.schema, t.handler)
	}

	// ── Seed additional workflows ───────────────────────────────────
	type wf struct {
		slug, name, desc, steps string
	}
	workflows := []wf{
		{
			slug: "content-pipeline",
			name: "Content Pipeline",
			desc: "Draft content → review → improve → validate. Multi-stage content creation with quality gates.",
			steps: `[
				{"id": "draft", "type": "llm", "config": {"model": "gpt-4o-mini", "system": "You are a skilled content writer.", "prompt": "Write a concise, engaging article about: {{input}}"}},
				{"id": "review", "type": "llm", "depends_on": ["draft"], "config": {"model": "gpt-4o", "system": "You are a strict editor. Identify weaknesses, factual issues, and areas for improvement. Be specific.", "prompt": "Review this draft:\n\n{{steps.draft.output}}"}},
				{"id": "improve", "type": "llm", "depends_on": ["draft", "review"], "config": {"model": "gpt-4o", "system": "You are a skilled rewriter. Improve the draft based on the editor feedback.", "prompt": "Original draft:\n{{steps.draft.output}}\n\nEditor feedback:\n{{steps.review.output}}\n\nRewrite the article incorporating the feedback."}},
				{"id": "validate", "type": "gate", "depends_on": ["improve"], "config": {"condition": "not_empty", "prompt": "{{steps.improve.output}}"}}
			]`,
		},
		{
			slug: "translate-verify",
			name: "Translate & Verify",
			desc: "Translate text then back-translate to verify accuracy. Catches drift and mistranslation.",
			steps: `[
				{"id": "translate", "type": "llm", "config": {"model": "gpt-4o-mini", "system": "You are a professional translator. Translate the following text to Spanish. Output only the translation.", "prompt": "{{input}}"}},
				{"id": "backtranslate", "type": "llm", "depends_on": ["translate"], "config": {"model": "gpt-4o-mini", "system": "You are a professional translator. Translate the following Spanish text back to English. Output only the translation.", "prompt": "{{steps.translate.output}}"}},
				{"id": "compare", "type": "llm", "depends_on": ["backtranslate"], "config": {"model": "gpt-4o", "system": "Compare the original and back-translated texts. Score similarity 1-10 and list any meaning changes. Respond as JSON: {\"score\": N, \"changes\": [...]}", "prompt": "Original:\n{{input}}\n\nBack-translated:\n{{steps.backtranslate.output}}"}},
				{"id": "result", "type": "transform", "depends_on": ["compare"], "config": {"expression": "extract_json"}}
			]`,
		},
		{
			slug: "multi-model-compare",
			name: "Multi-Model Compare",
			desc: "Send the same prompt to two models and compare responses. Great for model selection.",
			steps: `[
				{"id": "model_a", "type": "llm", "config": {"model": "gpt-4o-mini", "prompt": "{{input}}"}},
				{"id": "model_b", "type": "llm", "config": {"model": "gpt-4o", "prompt": "{{input}}"}},
				{"id": "compare", "type": "llm", "depends_on": ["model_a", "model_b"], "config": {"model": "gpt-4o", "system": "Compare these two AI responses objectively. Which is better and why? Score each 1-10. Respond as JSON.", "prompt": "Prompt: {{input}}\n\nResponse A (gpt-4o-mini):\n{{steps.model_a.output}}\n\nResponse B (gpt-4o):\n{{steps.model_b.output}}"}},
				{"id": "result", "type": "transform", "depends_on": ["compare"], "config": {"expression": "extract_json"}}
			]`,
		},
		{
			slug: "summarize-and-extract",
			name: "Summarize & Extract",
			desc: "Summarize long text, then extract key entities and action items.",
			steps: `[
				{"id": "summarize", "type": "llm", "config": {"model": "gpt-4o-mini", "system": "Summarize the following text concisely, capturing all key points.", "prompt": "{{input}}"}},
				{"id": "extract", "type": "llm", "depends_on": ["summarize"], "config": {"model": "gpt-4o-mini", "system": "From this summary, extract: people mentioned, organizations, dates, action items, and key decisions. Respond as JSON.", "prompt": "{{steps.summarize.output}}"}},
				{"id": "result", "type": "transform", "depends_on": ["extract"], "config": {"expression": "extract_json"}}
			]`,
		},
		{
			slug: "code-review-pipeline",
			name: "Code Review Pipeline",
			desc: "Analyze code for bugs, security, performance, then produce a consolidated review.",
			steps: `[
				{"id": "bugs", "type": "llm", "config": {"model": "gpt-4o", "system": "You are a bug hunter. Find bugs, logic errors, and edge cases in this code. Be specific with line references.", "prompt": "{{input}}"}},
				{"id": "security", "type": "llm", "config": {"model": "gpt-4o", "system": "You are a security auditor. Identify security vulnerabilities: injection, auth issues, data exposure, etc.", "prompt": "{{input}}"}},
				{"id": "perf", "type": "llm", "config": {"model": "gpt-4o-mini", "system": "You are a performance engineer. Identify performance bottlenecks, unnecessary allocations, N+1 queries, etc.", "prompt": "{{input}}"}},
				{"id": "consolidate", "type": "llm", "depends_on": ["bugs", "security", "perf"], "config": {"model": "gpt-4o", "system": "Consolidate these three code reviews into a single prioritized report. Group by severity: critical, high, medium, low.", "prompt": "Bug Analysis:\n{{steps.bugs.output}}\n\nSecurity Audit:\n{{steps.security.output}}\n\nPerformance Review:\n{{steps.perf.output}}"}}
			]`,
		},
	}
	for _, w := range workflows {
		// Try insert first, then update if exists with empty steps
		res, _ := conn.Exec(`INSERT OR IGNORE INTO forge_workflows (slug, name, description, steps_json, trigger_type, enabled) VALUES (?,?,?,?,?,1)`,
			w.slug, w.name, w.desc, w.steps, "manual")
		if affected, _ := res.RowsAffected(); affected == 0 {
			// Already exists — update if steps are empty/missing
			conn.Exec(`UPDATE forge_workflows SET name = ?, description = ?, steps_json = ?, updated_at = datetime('now') WHERE slug = ? AND (steps_json = '[]' OR steps_json = '' OR steps_json IS NULL)`,
				w.name, w.desc, w.steps, w.slug)
		}
	}

	// Clean up any legacy dummy workflows that aren't in the seed
	conn.Exec(`DELETE FROM forge_workflows WHERE slug = 'persist-proof'`)

	log.Printf("[forge] seeded %d tools + %d workflows", len(tools), len(workflows))
}
