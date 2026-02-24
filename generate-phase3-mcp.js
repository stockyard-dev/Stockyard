#!/usr/bin/env node
/**
 * Generate MCP server packages for Phase 3 P1 products.
 * Run: node generate-phase3-mcp.js
 */

const fs = require('fs');
const path = require('path');

const BASE = path.join(__dirname, 'mcp');

const PRODUCTS = [
  {
    key: "toxicfilter",
    binary: "toxicfilter",
    port: 5600,
    displayName: "ToxicFilter",
    tagline: "Content moderation for LLM outputs",
    description: "Content moderation middleware for LLM responses. Block, redact, or flag harmful, hateful, or unsafe content before it reaches users.",
    keywords: ["llm", "moderation", "safety", "toxic", "content-filter", "harmful", "proxy"],
    icon: "🛡️",
    defaultConfig: {
      port: 5600, data_dir: "~/.stockyard", log_level: "info", product: "toxicfilter",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      toxicfilter: { action: "flag", scan_output: true, categories: [{ name: "harmful", enabled: true, action: "block" }, { name: "hate_speech", enabled: true, action: "block" }] },
    },
    tools: [
      { name: "toxicfilter_stats", description: "Get moderation statistics: total scanned, blocks, flags, breakdown by category.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/toxic/stats" },
      { name: "toxicfilter_test", description: "Test a text string against moderation rules without sending to LLM.", inputSchema: { type: "object", properties: { text: { type: "string" } }, required: ["text"] }, apiPath: "/api/toxic/test", method: "POST" },
      { name: "toxicfilter_set_action", description: "Change the default moderation action.", inputSchema: { type: "object", properties: { action: { type: "string", enum: ["block", "flag", "redact", "log"] } }, required: ["action"] }, apiPath: "/api/toxic/action", method: "POST" },
      { name: "toxicfilter_categories", description: "List active moderation categories and their rules.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/toxic/categories" },
      { name: "toxicfilter_proxy_status", description: "Check if the ToxicFilter proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "compliancelog",
    binary: "compliancelog",
    port: 5610,
    displayName: "ComplianceLog",
    tagline: "Immutable audit trail for every LLM call",
    description: "Tamper-proof audit logging for LLM interactions. Hash-chained entries, configurable retention, SOC2/HIPAA-ready export formats.",
    keywords: ["llm", "audit", "compliance", "soc2", "hipaa", "logging", "immutable", "proxy"],
    icon: "📋",
    defaultConfig: {
      port: 5610, data_dir: "~/.stockyard", log_level: "info", product: "compliancelog",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      compliancelog: { hash_algorithm: "sha256", retention_days: 90, export_formats: ["json", "csv"], include_bodies: true },
    },
    tools: [
      { name: "compliancelog_stats", description: "Get audit log statistics: total entries, storage size, oldest/newest entry.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/compliance/stats" },
      { name: "compliancelog_search", description: "Search audit logs by date range, model, user, or project.", inputSchema: { type: "object", properties: { start: { type: "string", description: "ISO date" }, end: { type: "string", description: "ISO date" }, user: { type: "string" }, model: { type: "string" }, limit: { type: "number", default: 50 } } }, apiPath: "/api/compliance/search" },
      { name: "compliancelog_verify", description: "Verify hash chain integrity. Detects tampering.", inputSchema: { type: "object", properties: { start_id: { type: "number" }, end_id: { type: "number" } } }, apiPath: "/api/compliance/verify" },
      { name: "compliancelog_export", description: "Export audit logs in compliance format.", inputSchema: { type: "object", properties: { format: { type: "string", enum: ["json", "csv"], default: "json" }, start: { type: "string" }, end: { type: "string" } } }, apiPath: "/api/compliance/export" },
      { name: "compliancelog_proxy_status", description: "Check if the ComplianceLog proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "secretscan",
    binary: "secretscan",
    port: 5620,
    displayName: "SecretScan",
    tagline: "Catch API keys leaking through LLM calls",
    description: "Detect and redact API keys, AWS credentials, tokens, and secrets in LLM requests and responses. TruffleHog-style pattern matching.",
    keywords: ["llm", "security", "secrets", "api-key", "credential", "leak", "redaction", "proxy"],
    icon: "🔐",
    defaultConfig: {
      port: 5620, data_dir: "~/.stockyard", log_level: "info", product: "secretscan",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      secretscan: { scan_input: true, scan_output: true, action: "redact", patterns: ["aws_key", "github_pat", "openai_key", "anthropic_key", "stripe_key", "private_key"] },
    },
    tools: [
      { name: "secretscan_stats", description: "Get scan statistics: total scanned, detections by type, blocks/redactions.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/secrets/stats" },
      { name: "secretscan_test", description: "Test a text string for secret patterns without sending to LLM.", inputSchema: { type: "object", properties: { text: { type: "string" } }, required: ["text"] }, apiPath: "/api/secrets/test", method: "POST" },
      { name: "secretscan_patterns", description: "List active secret detection patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/secrets/patterns" },
      { name: "secretscan_set_action", description: "Change what happens when a secret is detected.", inputSchema: { type: "object", properties: { action: { type: "string", enum: ["block", "redact", "warn", "log"] } }, required: ["action"] }, apiPath: "/api/secrets/action", method: "POST" },
      { name: "secretscan_recent", description: "List recent secret detections.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/secrets/recent" },
      { name: "secretscan_proxy_status", description: "Check if the SecretScan proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "tracelink",
    binary: "tracelink",
    port: 5630,
    displayName: "TraceLink",
    tagline: "Distributed tracing for LLM chains",
    description: "Link related LLM calls into trace trees. Correlate multi-step agent workflows. OpenTelemetry-compatible trace propagation with waterfall visualization.",
    keywords: ["llm", "tracing", "observability", "opentelemetry", "distributed", "agent", "correlation", "proxy"],
    icon: "🔗",
    defaultConfig: {
      port: 5630, data_dir: "~/.stockyard", log_level: "info", product: "tracelink",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      tracelink: { sample_rate: 1.0, propagate_w3c: true, service_name: "my-app", max_spans: 10000 },
    },
    tools: [
      { name: "tracelink_traces", description: "List recent traces with root span info, total duration, and span count.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/traces" },
      { name: "tracelink_get", description: "Get full trace tree with all spans for a trace ID.", inputSchema: { type: "object", properties: { trace_id: { type: "string" } }, required: ["trace_id"] }, apiPath: "/api/traces/detail" },
      { name: "tracelink_stats", description: "Get tracing statistics: total traces, avg spans per trace, avg duration.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/traces/stats" },
      { name: "tracelink_search", description: "Search traces by model, duration, or span count.", inputSchema: { type: "object", properties: { model: { type: "string" }, min_duration_ms: { type: "number" }, min_spans: { type: "number" } } }, apiPath: "/api/traces/search" },
      { name: "tracelink_proxy_status", description: "Check if the TraceLink proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "alertpulse",
    binary: "alertpulse",
    port: 5640,
    displayName: "AlertPulse",
    tagline: "PagerDuty for your LLM stack",
    description: "Configurable alerting for LLM infrastructure. Rules for error rates, latency, cost thresholds. Notify via Slack, Discord, PagerDuty, email, or webhooks.",
    keywords: ["llm", "alerting", "monitoring", "slack", "pagerduty", "webhook", "threshold", "proxy"],
    icon: "🚨",
    defaultConfig: {
      port: 5640, data_dir: "~/.stockyard", log_level: "info", product: "alertpulse",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      alertpulse: { rules: [{ name: "high-error-rate", metric: "error_rate", threshold: 0.1, window: "5m", action: "webhook" }], channels: [{ type: "webhook", url: "${ALERT_WEBHOOK_URL}" }] },
    },
    tools: [
      { name: "alertpulse_rules", description: "List all alert rules with current status (firing/OK).", inputSchema: { type: "object", properties: {} }, apiPath: "/api/alerts/rules" },
      { name: "alertpulse_add_rule", description: "Add a new alert rule.", inputSchema: { type: "object", properties: { name: { type: "string" }, metric: { type: "string", enum: ["error_rate", "latency_p95", "cost_per_hour", "requests_per_minute"] }, operator: { type: "string", enum: [">", "<", ">=", "<="], default: ">" }, threshold: { type: "number" }, window: { type: "string", default: "5m" } }, required: ["name", "metric", "threshold"] }, apiPath: "/api/alerts/rules", method: "POST" },
      { name: "alertpulse_history", description: "Get alert history: recent firings and resolutions.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/alerts/history" },
      { name: "alertpulse_test", description: "Fire a test alert to verify notification channels work.", inputSchema: { type: "object", properties: { channel: { type: "string" } } }, apiPath: "/api/alerts/test", method: "POST" },
      { name: "alertpulse_proxy_status", description: "Check if the AlertPulse proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "chatmem",
    binary: "chatmem",
    port: 5650,
    displayName: "ChatMem",
    tagline: "Persistent conversation memory without token bloat",
    description: "Conversation memory middleware. Sliding window, summarization, and importance-based strategies. Persist memory across sessions without eating context windows.",
    keywords: ["llm", "memory", "conversation", "context", "session", "chatbot", "persistence", "proxy"],
    icon: "🧠",
    defaultConfig: {
      port: 5650, data_dir: "~/.stockyard", log_level: "info", product: "chatmem",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      chatmem: { strategy: "sliding_window", max_messages: 20, max_tokens: 4000, persistence: true },
    },
    tools: [
      { name: "chatmem_sessions", description: "List active conversation sessions with message counts and last activity.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/memory/sessions" },
      { name: "chatmem_get", description: "Get memory state for a specific session.", inputSchema: { type: "object", properties: { session_id: { type: "string" } }, required: ["session_id"] }, apiPath: "/api/memory/session" },
      { name: "chatmem_clear", description: "Clear memory for a specific session.", inputSchema: { type: "object", properties: { session_id: { type: "string" } }, required: ["session_id"] }, apiPath: "/api/memory/clear", method: "POST" },
      { name: "chatmem_stats", description: "Get memory statistics: active sessions, total messages stored, token savings.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/memory/stats" },
      { name: "chatmem_set_strategy", description: "Change memory management strategy.", inputSchema: { type: "object", properties: { strategy: { type: "string", enum: ["sliding_window", "summarization", "importance"] } }, required: ["strategy"] }, apiPath: "/api/memory/strategy", method: "POST" },
      { name: "chatmem_proxy_status", description: "Check if the ChatMem proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "mockllm",
    binary: "mockllm",
    port: 5660,
    displayName: "MockLLM",
    tagline: "Deterministic LLM responses for testing",
    description: "Mock LLM server with canned responses for CI/CD pipelines. Define fixtures, simulate errors, control latency. Never hit real APIs in tests.",
    keywords: ["llm", "mock", "testing", "ci-cd", "fixtures", "deterministic", "simulation", "proxy"],
    icon: "🧪",
    defaultConfig: {
      port: 5660, data_dir: "~/.stockyard", log_level: "info", product: "mockllm",
      providers: {},
      mockllm: { mode: "fixture", fixtures: [{ match: ".*", response: "This is a mock response.", delay: "100ms" }] },
    },
    tools: [
      { name: "mockllm_fixtures", description: "List all configured mock fixtures with match patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/mock/fixtures" },
      { name: "mockllm_add_fixture", description: "Add a new mock fixture. Matched prompts return the canned response.", inputSchema: { type: "object", properties: { match: { type: "string", description: "Regex pattern to match against prompts" }, response: { type: "string" }, model: { type: "string" }, delay: { type: "string", default: "100ms" }, status: { type: "number", description: "HTTP status code to return", default: 200 } }, required: ["match", "response"] }, apiPath: "/api/mock/fixtures", method: "POST" },
      { name: "mockllm_remove_fixture", description: "Remove a mock fixture by ID.", inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] }, apiPath: "/api/mock/fixtures/remove", method: "POST" },
      { name: "mockllm_stats", description: "Get mock server stats: total requests, fixture match rate, error simulations.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/mock/stats" },
      { name: "mockllm_set_mode", description: "Switch mock mode: fixture (canned), passthrough (forward to real), record (capture + replay).", inputSchema: { type: "object", properties: { mode: { type: "string", enum: ["fixture", "passthrough", "record"] } }, required: ["mode"] }, apiPath: "/api/mock/mode", method: "POST" },
      { name: "mockllm_proxy_status", description: "Check if the MockLLM proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "tenantwall",
    binary: "tenantwall",
    port: 5670,
    displayName: "TenantWall",
    tagline: "Per-tenant isolation for multi-tenant LLM apps",
    description: "Tenant isolation middleware. Per-tenant rate limits, spend caps, model access, and cache isolation. Build multi-tenant AI products without custom infrastructure.",
    keywords: ["llm", "multi-tenant", "saas", "isolation", "rate-limit", "per-tenant", "b2b", "proxy"],
    icon: "🏢",
    defaultConfig: {
      port: 5670, data_dir: "~/.stockyard", log_level: "info", product: "tenantwall",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      tenantwall: { tenant_header: "X-Tenant-Id", default_limits: { requests_per_minute: 30, daily_spend: 10.0 } },
    },
    tools: [
      { name: "tenantwall_tenants", description: "List all known tenants with usage summary.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/tenants" },
      { name: "tenantwall_tenant_usage", description: "Get detailed usage for a specific tenant.", inputSchema: { type: "object", properties: { tenant_id: { type: "string" } }, required: ["tenant_id"] }, apiPath: "/api/tenants/usage" },
      { name: "tenantwall_set_limits", description: "Set rate limits and spend caps for a tenant.", inputSchema: { type: "object", properties: { tenant_id: { type: "string" }, requests_per_minute: { type: "number" }, daily_spend: { type: "number" }, models: { type: "array", items: { type: "string" }, description: "Allowed models" } }, required: ["tenant_id"] }, apiPath: "/api/tenants/limits", method: "POST" },
      { name: "tenantwall_block", description: "Block a tenant from making LLM requests.", inputSchema: { type: "object", properties: { tenant_id: { type: "string" } }, required: ["tenant_id"] }, apiPath: "/api/tenants/block", method: "POST" },
      { name: "tenantwall_stats", description: "Get multi-tenant statistics: active tenants, total spend, top consumers.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/tenants/stats" },
      { name: "tenantwall_proxy_status", description: "Check if the TenantWall proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "idlekill",
    binary: "idlekill",
    port: 5680,
    displayName: "IdleKill",
    tagline: "Kill runaway LLM requests before they drain your wallet",
    description: "Request watchdog middleware. Kill LLM requests exceeding time, token, or cost limits. Stop agent loops, hanging streams, and runaway completions.",
    keywords: ["llm", "timeout", "watchdog", "runaway", "cost", "agent", "kill", "proxy"],
    icon: "⏱️",
    defaultConfig: {
      port: 5680, data_dir: "~/.stockyard", log_level: "info", product: "idlekill",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      idlekill: { max_duration: "120s", max_tokens: 16000, max_cost: 1.0, action: "kill", webhook: "" },
    },
    tools: [
      { name: "idlekill_stats", description: "Get watchdog statistics: total monitored, killed, reasons for kills.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/idlekill/stats" },
      { name: "idlekill_active", description: "List currently active LLM requests being monitored.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/idlekill/active" },
      { name: "idlekill_set_limits", description: "Update kill thresholds.", inputSchema: { type: "object", properties: { max_duration: { type: "string" }, max_tokens: { type: "number" }, max_cost: { type: "number" } } }, apiPath: "/api/idlekill/limits", method: "POST" },
      { name: "idlekill_recent_kills", description: "List recently killed requests with reasons.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/idlekill/kills" },
      { name: "idlekill_proxy_status", description: "Check if the IdleKill proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "ipfence",
    binary: "ipfence",
    port: 5690,
    displayName: "IPFence",
    tagline: "IP allowlisting for your LLM endpoints",
    description: "IP-level access control for LLM proxy endpoints. Allowlist, denylist, CIDR ranges. Block unauthorized access before any request processing.",
    keywords: ["llm", "security", "ip", "allowlist", "firewall", "access-control", "cidr", "proxy"],
    icon: "🧱",
    defaultConfig: {
      port: 5690, data_dir: "~/.stockyard", log_level: "info", product: "ipfence",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      ipfence: { mode: "allowlist", action: "block", allowlist: ["127.0.0.1/8", "10.0.0.0/8", "192.168.0.0/16"], trust_proxy: false },
    },
    tools: [
      { name: "ipfence_stats", description: "Get access control statistics: requests checked, blocked, allowed, unique IPs.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/ipfence/stats" },
      { name: "ipfence_add_allow", description: "Add an IP or CIDR range to the allowlist.", inputSchema: { type: "object", properties: { ip: { type: "string", description: "IP address or CIDR range (e.g. 10.0.0.0/8)" } }, required: ["ip"] }, apiPath: "/api/ipfence/allow", method: "POST" },
      { name: "ipfence_add_deny", description: "Add an IP or CIDR range to the denylist.", inputSchema: { type: "object", properties: { ip: { type: "string" } }, required: ["ip"] }, apiPath: "/api/ipfence/deny", method: "POST" },
      { name: "ipfence_check", description: "Check if a specific IP would be allowed or blocked.", inputSchema: { type: "object", properties: { ip: { type: "string" } }, required: ["ip"] }, apiPath: "/api/ipfence/check" },
      { name: "ipfence_recent", description: "List recent access events (allowed and blocked).", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/ipfence/recent" },
      { name: "ipfence_proxy_status", description: "Check if the IPFence proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "embedcache",
    binary: "embedcache",
    port: 5700,
    displayName: "EmbedCache",
    tagline: "Never compute the same embedding twice",
    description: "Embedding response caching for /v1/embeddings. Content-hash deduplication, per-input cache splitting, 7-day TTL. Slash embedding costs for RAG pipelines.",
    keywords: ["llm", "embedding", "cache", "rag", "vector", "deduplication", "cost", "proxy"],
    icon: "💎",
    defaultConfig: {
      port: 5700, data_dir: "~/.stockyard", log_level: "info", product: "embedcache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      embedcache: { max_entries: 100000, ttl: "168h" },
    },
    tools: [
      { name: "embedcache_stats", description: "Get cache statistics: entries, hit rate, bytes saved, evictions.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/embedcache/stats" },
      { name: "embedcache_flush", description: "Clear the embedding cache.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/embedcache/flush", method: "POST" },
      { name: "embedcache_lookup", description: "Check if a specific text has a cached embedding.", inputSchema: { type: "object", properties: { text: { type: "string" }, model: { type: "string", default: "text-embedding-3-small" } }, required: ["text"] }, apiPath: "/api/embedcache/lookup", method: "POST" },
      { name: "embedcache_set_ttl", description: "Change cache TTL for new entries.", inputSchema: { type: "object", properties: { ttl: { type: "string", description: "Duration (e.g. '168h', '30d')" } }, required: ["ttl"] }, apiPath: "/api/embedcache/ttl", method: "POST" },
      { name: "embedcache_proxy_status", description: "Check if the EmbedCache proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
  {
    key: "anthrofit",
    binary: "anthrofit",
    port: 5710,
    displayName: "AnthroFit",
    tagline: "Use Claude with OpenAI SDKs",
    description: "Deep Anthropic compatibility layer. System prompt consolidation, max_tokens injection, tool schema translation, streaming normalization. Drop-in Claude support for OpenAI apps.",
    keywords: ["llm", "anthropic", "claude", "openai", "compatibility", "adapter", "translation", "proxy"],
    icon: "🔄",
    defaultConfig: {
      port: 5710, data_dir: "~/.stockyard", log_level: "info", product: "anthrofit",
      providers: { anthropic: { api_key: "${ANTHROPIC_API_KEY}", base_url: "https://api.anthropic.com" } },
      anthrofit: { system_prompt_mode: "auto", tool_translation: true, stream_normalize: true, max_tokens_default: 4096 },
    },
    tools: [
      { name: "anthrofit_stats", description: "Get translation statistics: requests processed, system prompts fixed, tools translated, errors.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/anthrofit/stats" },
      { name: "anthrofit_test", description: "Test OpenAI→Anthropic translation on a request without sending.", inputSchema: { type: "object", properties: { model: { type: "string", default: "claude-sonnet-4-20250514" }, messages: { type: "array", items: { type: "object" } } }, required: ["messages"] }, apiPath: "/api/anthrofit/test", method: "POST" },
      { name: "anthrofit_set_mode", description: "Change system prompt handling mode.", inputSchema: { type: "object", properties: { mode: { type: "string", enum: ["auto", "separate", "merge"] } }, required: ["mode"] }, apiPath: "/api/anthrofit/mode", method: "POST" },
      { name: "anthrofit_config", description: "Get current AnthroFit configuration.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/anthrofit/config" },
      { name: "anthrofit_proxy_status", description: "Check if the AnthroFit proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
];

// ─── Generate package files ────────────────────────────────────────────

for (const prod of PRODUCTS) {
  const dir = path.join(BASE, 'packages', `mcp-${prod.key}`);
  fs.mkdirSync(dir, { recursive: true });

  // index.js
  fs.writeFileSync(path.join(dir, 'index.js'), `#!/usr/bin/env node
/**
 * @stockyard/mcp-${prod.key} — ${prod.tagline}
 * 
 * MCP server for Stockyard ${prod.displayName}.
 * ${prod.description.split('.')[0]}.
 * 
 * Usage with Claude Desktop / Cursor / Windsurf:
 *   npx @stockyard/mcp-${prod.key}
 * 
 * Or add to your MCP config:
 *   { "command": "npx", "args": ["@stockyard/mcp-${prod.key}"] }
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("${prod.key}");
server.start();
`);

  // package.json
  fs.writeFileSync(path.join(dir, 'package.json'), JSON.stringify({
    name: `@stockyard/mcp-${prod.key}`,
    version: "0.1.0",
    description: prod.description,
    main: "index.js",
    bin: { [`mcp-stockyard-${prod.key}`]: "index.js" },
    keywords: ["mcp", "mcp-server", ...prod.keywords, "stockyard", "model-context-protocol", "cursor", "claude-desktop"],
    author: "Stockyard",
    license: "MIT",
    repository: { type: "git", url: `https://github.com/stockyard-dev/mcp-${prod.key}` },
    homepage: `https://stockyard.dev/mcp/${prod.key}`,
    engines: { node: ">=18" },
    os: ["darwin", "linux", "win32"],
    cpu: ["x64", "arm64"],
    files: ["index.js", "README.md"],
  }, null, 2) + '\n');

  // smithery.yaml
  fs.writeFileSync(path.join(dir, 'smithery.yaml'), `# Smithery Marketplace Configuration
name: stockyard-${prod.key}
display_name: "Stockyard ${prod.displayName}"
description: "${prod.description}"
icon: "${prod.icon}"
command: npx
args:
  - "@stockyard/mcp-${prod.key}"
env:
  ${prod.key === 'anthrofit' ? 'ANTHROPIC_API_KEY' : 'OPENAI_API_KEY'}:
    description: "${prod.key === 'anthrofit' ? 'Anthropic' : 'OpenAI'} API key"
    required: true
    secret: true
tags:
${prod.keywords.map(k => `  - ${k}`).join('\n')}
`);

  // glama.json
  const glamaTools = prod.tools.map(t => ({ name: t.name, description: t.description }));
  fs.writeFileSync(path.join(dir, 'glama.json'), JSON.stringify({
    name: `stockyard-${prod.key}`,
    display_name: `Stockyard ${prod.displayName}`,
    description: prod.description,
    repository: `https://github.com/stockyard-dev/mcp-${prod.key}`,
    command: "npx",
    args: [`@stockyard/mcp-${prod.key}`],
    tools: glamaTools,
    tags: prod.keywords,
  }, null, 2) + '\n');

  // mcp-so.json
  fs.writeFileSync(path.join(dir, 'mcp-so.json'), JSON.stringify({
    name: `@stockyard/mcp-${prod.key}`,
    title: `${prod.displayName} — ${prod.tagline}`,
    description: prod.description,
    install: `npx @stockyard/mcp-${prod.key}`,
    config: {
      command: "npx",
      args: [`@stockyard/mcp-${prod.key}`],
      env: prod.key === 'anthrofit'
        ? { ANTHROPIC_API_KEY: "your-key" }
        : prod.key === 'mockllm'
          ? {}
          : { OPENAI_API_KEY: "your-key" },
    },
    tools_count: prod.tools.length,
    categories: ["llm", "proxy", "developer-tools"],
  }, null, 2) + '\n');

  // README.md
  const envKey = prod.key === 'anthrofit' ? 'ANTHROPIC_API_KEY' : 'OPENAI_API_KEY';
  const envKeyPlaceholder = prod.key === 'anthrofit' ? 'sk-ant-your-key-here' : 'sk-your-key-here';
  const toolPrompts = prod.tools.map(t => {
    const shortDesc = t.description.split('.')[0];
    return `- **"${shortDesc}"**`;
  });

  fs.writeFileSync(path.join(dir, 'README.md'), `# @stockyard/mcp-${prod.key}

> ${prod.tagline}

**${prod.description.split('.')[0]} via MCP.**

## Quick Start

### Claude Desktop

Add to \`~/Library/Application Support/Claude/claude_desktop_config.json\`:

\`\`\`json
{
  "mcpServers": {
    "stockyard-${prod.key}": {
      "command": "npx",
      "args": ["@stockyard/mcp-${prod.key}"],
      "env": {
        "${envKey}": "${envKeyPlaceholder}"
      }
    }
  }
}
\`\`\`

### Cursor

Add to \`.cursor/mcp.json\`:

\`\`\`json
{
  "mcpServers": {
    "stockyard-${prod.key}": {
      "command": "npx",
      "args": ["@stockyard/mcp-${prod.key}"]
    }
  }
}
\`\`\`

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

\`\`\`json
{
  "mcpServers": {
    "stockyard-${prod.key}": {
      "command": "npx",
      "args": ["@stockyard/mcp-${prod.key}"]
    }
  }
}
\`\`\`

## Available Tools

Once connected, ask your AI assistant:

- **"Set up ${prod.displayName}"** — Downloads and starts the proxy
${toolPrompts.join('\n')}
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard \`${prod.binary}\` binary for your platform
2. It writes a config and starts the proxy on port ${prod.port}
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at \`http://127.0.0.1:${prod.port}/v1\` to route through ${prod.displayName}
5. Dashboard available at \`http://127.0.0.1:${prod.port}/ui\`

## Requirements

- Node.js 18+
${prod.key === 'mockllm' ? '- No API key required (MockLLM provides canned responses)' : `- An LLM API key (set \`${envKey}\`)`}

## Why ${prod.displayName}?

${prod.description}

## Part of Stockyard

${prod.displayName} is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \\$19/mo (saves 89% vs buying individually).

## License

MIT
`);

  console.log(`✓ mcp-${prod.key} (${prod.tools.length} tools)`);
}

// ─── Update products.js with Phase 3 P1 additions ────────────────────
console.log(`\n✓ Generated ${PRODUCTS.length} MCP packages`);
console.log(`  Total tools: ${PRODUCTS.reduce((a, p) => a + p.tools.length, 0)}`);
console.log(`\nNow update shared/products.js manually with the tool definitions.`);
