/**
 * Stockyard Product Definitions for MCP Servers
 * Each product defines: binary name, port, default config, and MCP tool definitions.
 * 125 products total across all phases
 */

const PRODUCTS = {
  // ═══════════════════════════════════════════════════════════════════
  // ORIGINAL 7 PRODUCTS
  // ═══════════════════════════════════════════════════════════════════

  costcap: {
    binary: "costcap",
    port: 4100,
    displayName: "CostCap",
    tagline: "Never get a surprise LLM bill again",
    description: "LLM spending caps and budget tracking. Set daily/monthly limits, get alerts, and auto-block when budgets are hit.",
    keywords: ["llm", "cost", "budget", "spending", "caps", "openai", "anthropic", "proxy"],
    defaultConfig: {
      port: 4100, data_dir: "~/.stockyard", log_level: "info", product: "costcap",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      projects: { default: { provider: "openai", model: "gpt-4o-mini", caps: { daily: 5.0, monthly: 50.0 }, alerts: { thresholds: [0.5, 0.8, 1.0] } } },
    },
    tools: [
      { name: "costcap_get_spend", description: "Get current LLM spending for a project. Returns today's spend, this month's spend, and remaining budget.", inputSchema: { type: "object", properties: { project: { type: "string", description: "Project name (default: 'default')", default: "default" } } }, apiPath: "/api/spend" },
      { name: "costcap_set_budget", description: "Set daily and/or monthly spending caps for a project.", inputSchema: { type: "object", properties: { project: { type: "string", default: "default" }, daily: { type: "number", description: "Daily spending cap in USD" }, monthly: { type: "number", description: "Monthly spending cap in USD" } } }, apiPath: "/api/budget", method: "POST" },
      { name: "costcap_get_usage", description: "Get detailed usage breakdown by model, showing token counts and costs.", inputSchema: { type: "object", properties: { project: { type: "string", default: "default" }, period: { type: "string", enum: ["today", "week", "month"], default: "today" } } }, apiPath: "/api/usage" },
      { name: "costcap_proxy_status", description: "Check if the CostCap proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  llmcache: {
    binary: "llmcache",
    port: 4200,
    displayName: "CacheLayer",
    tagline: "Stop paying twice for the same LLM response",
    description: "Intelligent LLM response caching. Exact-match caching with configurable TTL. Reduces costs and latency for repeated prompts.",
    keywords: ["llm", "cache", "caching", "cost", "latency", "openai", "proxy", "redis-free"],
    defaultConfig: {
      port: 4200, data_dir: "~/.stockyard", log_level: "info", product: "llmcache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      cache: { ttl: "1h", max_entries: 10000 },
    },
    tools: [
      { name: "cache_stats", description: "Get cache hit/miss statistics. Shows hit rate, total entries, and estimated savings.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/cache/stats" },
      { name: "cache_flush", description: "Clear the entire cache or cache entries matching a pattern.", inputSchema: { type: "object", properties: { pattern: { type: "string", description: "Optional: only flush entries matching this model name" } } }, apiPath: "/api/cache/flush", method: "POST" },
      { name: "cache_set_ttl", description: "Change the cache TTL (time-to-live) for new entries.", inputSchema: { type: "object", properties: { ttl: { type: "string", description: "Duration string, e.g. '30m', '2h', '1d'" } }, required: ["ttl"] }, apiPath: "/api/cache/ttl", method: "POST" },
      { name: "cache_proxy_status", description: "Check if the CacheLayer proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  jsonguard: {
    binary: "jsonguard",
    port: 4300,
    displayName: "StructuredShield",
    tagline: "LLM responses that always parse",
    description: "JSON schema validation for LLM responses. Automatically retries when responses don't match your schema.",
    keywords: ["llm", "json", "schema", "validation", "structured", "output", "openai", "proxy"],
    defaultConfig: {
      port: 4300, data_dir: "~/.stockyard", log_level: "info", product: "jsonguard",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      validation: { max_retries: 3, strict: true },
    },
    tools: [
      { name: "jsonguard_stats", description: "Get validation statistics: total requests, pass rate, retry rate, common failure patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/validation/stats" },
      { name: "jsonguard_set_schema", description: "Register a named JSON schema for automatic validation of responses.", inputSchema: { type: "object", properties: { name: { type: "string", description: "Schema name" }, schema: { type: "object", description: "JSON Schema object" } }, required: ["name", "schema"] }, apiPath: "/api/validation/schema", method: "POST" },
      { name: "jsonguard_proxy_status", description: "Check if the StructuredShield proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  routefall: {
    binary: "routefall",
    port: 4400,
    displayName: "FallbackRouter",
    tagline: "LLM calls that never fail",
    description: "Automatic failover routing between LLM providers. When OpenAI is down, traffic auto-routes to Anthropic or Groq. Circuit breaker pattern included.",
    keywords: ["llm", "failover", "fallback", "routing", "circuit-breaker", "openai", "anthropic", "proxy"],
    defaultConfig: {
      port: 4400, data_dir: "~/.stockyard", log_level: "info", product: "routefall",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" }, anthropic: { api_key: "${ANTHROPIC_API_KEY}" }, groq: { api_key: "${GROQ_API_KEY}" } },
      routing: { strategy: "failover", primary: "openai", fallbacks: ["anthropic", "groq"], circuit_breaker: { threshold: 3, timeout: "30s" } },
    },
    tools: [
      { name: "routefall_provider_status", description: "Get health status of all configured LLM providers.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/providers/status" },
      { name: "routefall_routing_stats", description: "Get routing statistics: requests per provider, failover counts, circuit breaker trips.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/routing/stats" },
      { name: "routefall_set_primary", description: "Change the primary provider for routing.", inputSchema: { type: "object", properties: { provider: { type: "string", description: "Provider name" } }, required: ["provider"] }, apiPath: "/api/routing/primary", method: "POST" },
      { name: "routefall_proxy_status", description: "Check if the FallbackRouter proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  rateshield: {
    binary: "rateshield",
    port: 4500,
    displayName: "RateShield",
    tagline: "Bulletproof your LLM rate limits",
    description: "Rate limiting and request queuing for LLM APIs. Token bucket algorithm with configurable limits per-key, per-model, or per-user.",
    keywords: ["llm", "rate-limit", "throttle", "queue", "token-bucket", "openai", "proxy"],
    defaultConfig: {
      port: 4500, data_dir: "~/.stockyard", log_level: "info", product: "rateshield",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      rate_limits: { requests_per_minute: 60, tokens_per_minute: 100000, strategy: "token_bucket" },
    },
    tools: [
      { name: "rateshield_limit_status", description: "Get current rate limit status: remaining requests, tokens, reset time.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/ratelimit/status" },
      { name: "rateshield_set_limits", description: "Update rate limits dynamically.", inputSchema: { type: "object", properties: { requests_per_minute: { type: "number" }, tokens_per_minute: { type: "number" } } }, apiPath: "/api/ratelimit/config", method: "POST" },
      { name: "rateshield_queue_stats", description: "Get request queue statistics: queued, processing, rejected, avg wait time.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/queue/stats" },
      { name: "rateshield_proxy_status", description: "Check if the RateShield proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptreplay: {
    binary: "promptreplay",
    port: 4600,
    displayName: "PromptReplay",
    tagline: "Every LLM call, logged and replayable",
    description: "Full request/response logging for LLM APIs. Capture every prompt, completion, and token count. Replay past requests for debugging.",
    keywords: ["llm", "logging", "replay", "debug", "prompt", "audit", "openai", "proxy"],
    defaultConfig: {
      port: 4600, data_dir: "~/.stockyard", log_level: "info", product: "promptreplay",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      logging: { full_body: true, retention: "7d" },
    },
    tools: [
      { name: "promptreplay_list", description: "List recent logged LLM requests with timestamps, models, and costs.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 }, model: { type: "string" } } }, apiPath: "/api/logs" },
      { name: "promptreplay_get", description: "Get full details of a logged request including prompt and response.", inputSchema: { type: "object", properties: { id: { type: "string", description: "Log entry ID" } }, required: ["id"] }, apiPath: "/api/logs/detail" },
      { name: "promptreplay_replay", description: "Replay a previously logged LLM request.", inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] }, apiPath: "/api/logs/replay", method: "POST" },
      { name: "promptreplay_stats", description: "Get logging statistics: total entries, storage size, requests by model.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/logs/stats" },
      { name: "promptreplay_proxy_status", description: "Check if the PromptReplay proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  // ═══════════════════════════════════════════════════════════════════
  // PHASE 1 EXPANSION — 5 PRODUCTS
  // ═══════════════════════════════════════════════════════════════════

  keypool: {
    binary: "keypool",
    port: 4700,
    displayName: "KeyPool",
    tagline: "Pool your API keys, multiply your limits",
    description: "API key pooling and rotation for LLM providers. Round-robin, least-used, or random strategies. Auto-rotate on 429 rate limits.",
    keywords: ["llm", "api-key", "pool", "rotation", "rate-limit", "429", "openai", "proxy"],
    defaultConfig: {
      port: 4700, data_dir: "~/.stockyard", log_level: "info", product: "keypool",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      key_pool: { strategy: "round-robin", keys: [{ name: "key-1", provider: "openai", key: "${OPENAI_API_KEY_1}" }, { name: "key-2", provider: "openai", key: "${OPENAI_API_KEY_2}" }], auto_rotate_on_429: true, cooldown: "60s" },
    },
    tools: [
      { name: "keypool_status", description: "Get status of all pooled API keys: active, cooldown, usage counts, last error.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/keypool/status" },
      { name: "keypool_add_key", description: "Add a new API key to the pool.", inputSchema: { type: "object", properties: { name: { type: "string" }, provider: { type: "string" }, key: { type: "string" } }, required: ["name", "provider", "key"] }, apiPath: "/api/keypool/keys", method: "POST" },
      { name: "keypool_remove_key", description: "Remove an API key from the pool by name.", inputSchema: { type: "object", properties: { name: { type: "string" } }, required: ["name"] }, apiPath: "/api/keypool/keys/remove", method: "POST" },
      { name: "keypool_set_strategy", description: "Change the key rotation strategy.", inputSchema: { type: "object", properties: { strategy: { type: "string", enum: ["round-robin", "least-used", "random"] } }, required: ["strategy"] }, apiPath: "/api/keypool/strategy", method: "POST" },
      { name: "keypool_stats", description: "Get pool statistics: requests per key, 429 counts, rotation events.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/keypool/stats" },
      { name: "keypool_proxy_status", description: "Check if the KeyPool proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptguard: {
    binary: "promptguard",
    port: 4800,
    displayName: "PromptGuard",
    tagline: "PII never hits the LLM",
    description: "PII redaction and prompt injection detection for LLM APIs. Regex-based redaction with restore capability. Block or sanitize dangerous prompts.",
    keywords: ["llm", "pii", "redaction", "security", "injection", "privacy", "openai", "proxy"],
    defaultConfig: {
      port: 4800, data_dir: "~/.stockyard", log_level: "info", product: "promptguard",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      prompt_guard: { mode: "redact", pii_patterns: ["email", "phone", "ssn", "credit_card"], injection_detection: { enabled: true, sensitivity: "medium" } },
    },
    tools: [
      { name: "promptguard_stats", description: "Get guard statistics: total scanned, PII detections by type, injection blocks.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/guard/stats" },
      { name: "promptguard_test", description: "Test a prompt against PII detection and injection rules without sending to LLM.", inputSchema: { type: "object", properties: { text: { type: "string" } }, required: ["text"] }, apiPath: "/api/guard/test", method: "POST" },
      { name: "promptguard_set_mode", description: "Change the guard mode: redact, redact-restore, or block.", inputSchema: { type: "object", properties: { mode: { type: "string", enum: ["redact", "redact-restore", "block"] } }, required: ["mode"] }, apiPath: "/api/guard/mode", method: "POST" },
      { name: "promptguard_set_sensitivity", description: "Set injection detection sensitivity level.", inputSchema: { type: "object", properties: { sensitivity: { type: "string", enum: ["low", "medium", "high"] } }, required: ["sensitivity"] }, apiPath: "/api/guard/sensitivity", method: "POST" },
      { name: "promptguard_proxy_status", description: "Check if the PromptGuard proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  modelswitch: {
    binary: "modelswitch",
    port: 4900,
    displayName: "ModelSwitch",
    tagline: "Right model, right prompt, right price",
    description: "Smart model routing based on token count, prompt patterns, and headers. Route complex queries to GPT-4o and simple ones to GPT-4o-mini.",
    keywords: ["llm", "routing", "model", "smart", "cost", "optimization", "a/b-test", "proxy"],
    defaultConfig: {
      port: 4900, data_dir: "~/.stockyard", log_level: "info", product: "modelswitch",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      model_switch: { default_model: "gpt-4o-mini", rules: [{ name: "long-context", condition: "token_count > 2000", model: "gpt-4o" }] },
    },
    tools: [
      { name: "modelswitch_stats", description: "Get routing statistics: requests per model, cost per route, A/B test results.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/modelswitch/stats" },
      { name: "modelswitch_rules", description: "List current routing rules and their match counts.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/modelswitch/rules" },
      { name: "modelswitch_add_rule", description: "Add a new routing rule.", inputSchema: { type: "object", properties: { name: { type: "string" }, condition: { type: "string" }, model: { type: "string" } }, required: ["name", "condition", "model"] }, apiPath: "/api/modelswitch/rules", method: "POST" },
      { name: "modelswitch_test", description: "Test which model a prompt would be routed to.", inputSchema: { type: "object", properties: { text: { type: "string" } }, required: ["text"] }, apiPath: "/api/modelswitch/test", method: "POST" },
      { name: "modelswitch_proxy_status", description: "Check if the ModelSwitch proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  evalgate: {
    binary: "evalgate",
    port: 4110,
    displayName: "EvalGate",
    tagline: "Only ship quality LLM responses",
    description: "Response quality scoring and auto-retry. Validate JSON, check length, match regex, run custom expressions. Auto-retry on failure.",
    keywords: ["llm", "eval", "quality", "validation", "retry", "scoring", "openai", "proxy"],
    defaultConfig: {
      port: 4110, data_dir: "~/.stockyard", log_level: "info", product: "evalgate",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      eval_gate: { validators: [{ type: "json_parse" }, { type: "min_length", value: 50 }], retry: { max_retries: 2, budget_per_minute: 10 } },
    },
    tools: [
      { name: "evalgate_stats", description: "Get evaluation statistics: pass/fail rate, retry counts, validator breakdown.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/eval/stats" },
      { name: "evalgate_validators", description: "List active validators and their pass rates.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/eval/validators" },
      { name: "evalgate_add_validator", description: "Add a new response validator.", inputSchema: { type: "object", properties: { type: { type: "string", enum: ["json_parse", "min_length", "max_length", "regex_match", "regex_exclude"] }, value: { type: "string" } }, required: ["type"] }, apiPath: "/api/eval/validators", method: "POST" },
      { name: "evalgate_test", description: "Test a response string against all active validators.", inputSchema: { type: "object", properties: { text: { type: "string" } }, required: ["text"] }, apiPath: "/api/eval/test", method: "POST" },
      { name: "evalgate_proxy_status", description: "Check if the EvalGate proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  usagepulse: {
    binary: "usagepulse",
    port: 4410,
    displayName: "UsagePulse",
    tagline: "Know exactly where every token goes",
    description: "Per-user and per-feature token metering. Multi-dimensional tracking, spend caps, and billing export in CSV/JSON.",
    keywords: ["llm", "usage", "metering", "billing", "tokens", "analytics", "openai", "proxy"],
    defaultConfig: {
      port: 4410, data_dir: "~/.stockyard", log_level: "info", product: "usagepulse",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      usage_pulse: { dimensions: ["user", "feature"], caps: { per_user_daily: 10.0 }, export: { format: "json" } },
    },
    tools: [
      { name: "usagepulse_usage", description: "Get usage breakdown by dimension (user, feature, team).", inputSchema: { type: "object", properties: { dimension: { type: "string", enum: ["user", "feature", "team", "model"] }, period: { type: "string", enum: ["today", "week", "month"], default: "today" } } }, apiPath: "/api/usage/breakdown" },
      { name: "usagepulse_user_usage", description: "Get usage for a specific user.", inputSchema: { type: "object", properties: { user_id: { type: "string" } }, required: ["user_id"] }, apiPath: "/api/usage/user" },
      { name: "usagepulse_set_cap", description: "Set a spending cap for a user or team.", inputSchema: { type: "object", properties: { dimension: { type: "string", enum: ["user", "team"] }, id: { type: "string" }, daily: { type: "number" }, monthly: { type: "number" } }, required: ["dimension", "id"] }, apiPath: "/api/usage/caps", method: "POST" },
      { name: "usagepulse_export", description: "Export usage data as CSV or JSON for billing.", inputSchema: { type: "object", properties: { format: { type: "string", enum: ["csv", "json"], default: "json" }, period: { type: "string", enum: ["today", "week", "month"], default: "month" } } }, apiPath: "/api/usage/export" },
      { name: "usagepulse_proxy_status", description: "Check if the UsagePulse proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  // ═══════════════════════════════════════════════════════════════════
  // PHASE 2 EXPANSION — 8 PRODUCTS
  // ═══════════════════════════════════════════════════════════════════

  promptpad: {
    binary: "promptpad",
    port: 4801,
    displayName: "PromptPad",
    tagline: "Version control for your prompts",
    description: "Prompt template versioning and A/B testing. Store, version, and test prompt templates. Track which variants perform best.",
    keywords: ["llm", "prompt", "template", "versioning", "a/b-test", "management", "openai", "proxy"],
    defaultConfig: {
      port: 4801, data_dir: "~/.stockyard", log_level: "info", product: "promptpad",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      prompt_pad: { storage_dir: "~/.stockyard/templates", api_prefix: "/api/templates" },
    },
    tools: [
      { name: "promptpad_list", description: "List all prompt templates with version numbers and usage stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/templates" },
      { name: "promptpad_get", description: "Get a specific prompt template by name and optional version.", inputSchema: { type: "object", properties: { name: { type: "string" }, version: { type: "number" } }, required: ["name"] }, apiPath: "/api/templates/get" },
      { name: "promptpad_save", description: "Save or update a prompt template. Automatically versions.", inputSchema: { type: "object", properties: { name: { type: "string" }, template: { type: "string", description: "Prompt with {{variable}} placeholders" }, description: { type: "string" } }, required: ["name", "template"] }, apiPath: "/api/templates", method: "POST" },
      { name: "promptpad_render", description: "Render a template with variables and optionally send to LLM.", inputSchema: { type: "object", properties: { name: { type: "string" }, variables: { type: "object" }, send: { type: "boolean", default: false } }, required: ["name", "variables"] }, apiPath: "/api/templates/render", method: "POST" },
      { name: "promptpad_ab_stats", description: "Get A/B test results for prompt variants.", inputSchema: { type: "object", properties: { name: { type: "string" } }, required: ["name"] }, apiPath: "/api/templates/ab" },
      { name: "promptpad_proxy_status", description: "Check if the PromptPad proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  tokentrim: {
    binary: "tokentrim",
    port: 4901,
    displayName: "TokenTrim",
    tagline: "Never hit a context limit again",
    description: "Automatic context window management. Truncates prompts using middle-out, oldest-first, or newest-first strategies when they exceed model limits.",
    keywords: ["llm", "token", "context", "truncation", "window", "optimization", "openai", "proxy"],
    defaultConfig: {
      port: 4901, data_dir: "~/.stockyard", log_level: "info", product: "tokentrim",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      token_trim: { strategy: "middle-out", safety_margin: 500, model_limits: { "gpt-4o": 128000, "gpt-4o-mini": 128000, "gpt-3.5-turbo": 16385 } },
    },
    tools: [
      { name: "tokentrim_stats", description: "Get trimming statistics: total trimmed, tokens saved, trim rate.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/trim/stats" },
      { name: "tokentrim_count", description: "Count tokens in a text string without sending to LLM.", inputSchema: { type: "object", properties: { text: { type: "string" }, model: { type: "string", default: "gpt-4o" } }, required: ["text"] }, apiPath: "/api/trim/count", method: "POST" },
      { name: "tokentrim_set_strategy", description: "Change the truncation strategy.", inputSchema: { type: "object", properties: { strategy: { type: "string", enum: ["middle-out", "oldest-first", "newest-first"] } }, required: ["strategy"] }, apiPath: "/api/trim/strategy", method: "POST" },
      { name: "tokentrim_set_limit", description: "Set a custom token limit for a model.", inputSchema: { type: "object", properties: { model: { type: "string" }, limit: { type: "number" } }, required: ["model", "limit"] }, apiPath: "/api/trim/limits", method: "POST" },
      { name: "tokentrim_proxy_status", description: "Check if the TokenTrim proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  batchqueue: {
    binary: "batchqueue",
    port: 5000,
    displayName: "BatchQueue",
    tagline: "Background jobs for LLM calls",
    description: "Async job queue for LLM requests with priority levels, concurrency control, and retry. Queue thousands of requests and process them reliably.",
    keywords: ["llm", "batch", "queue", "async", "job", "background", "concurrency", "proxy"],
    defaultConfig: {
      port: 5000, data_dir: "~/.stockyard", log_level: "info", product: "batchqueue",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      batch_queue: { concurrency: { openai: 5, anthropic: 3 }, retry: { max_retries: 3, backoff: "exponential" }, priority_levels: ["critical", "high", "normal", "low"] },
    },
    tools: [
      { name: "batchqueue_submit", description: "Submit a new LLM request to the queue. Returns a job ID.", inputSchema: { type: "object", properties: { model: { type: "string" }, messages: { type: "array", items: { type: "object" } }, priority: { type: "string", enum: ["critical", "high", "normal", "low"], default: "normal" }, callback_url: { type: "string" } }, required: ["model", "messages"] }, apiPath: "/api/queue/submit", method: "POST" },
      { name: "batchqueue_status", description: "Get status of a queued job by ID.", inputSchema: { type: "object", properties: { job_id: { type: "string" } }, required: ["job_id"] }, apiPath: "/api/queue/status" },
      { name: "batchqueue_result", description: "Get the result of a completed job.", inputSchema: { type: "object", properties: { job_id: { type: "string" } }, required: ["job_id"] }, apiPath: "/api/queue/result" },
      { name: "batchqueue_queue_stats", description: "Get queue statistics: pending, processing, completed, failed.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/queue/stats" },
      { name: "batchqueue_cancel", description: "Cancel a pending or processing job.", inputSchema: { type: "object", properties: { job_id: { type: "string" } }, required: ["job_id"] }, apiPath: "/api/queue/cancel", method: "POST" },
      { name: "batchqueue_proxy_status", description: "Check if the BatchQueue proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  multicall: {
    binary: "multicall",
    port: 5100,
    displayName: "MultiCall",
    tagline: "Ask multiple models, pick the best answer",
    description: "Send the same prompt to multiple LLMs simultaneously. Pick the fastest, cheapest, longest, or consensus response.",
    keywords: ["llm", "multi-model", "consensus", "comparison", "a/b-test", "routing", "proxy"],
    defaultConfig: {
      port: 5100, data_dir: "~/.stockyard", log_level: "info", product: "multicall",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" }, anthropic: { api_key: "${ANTHROPIC_API_KEY}" } },
      multi_call: { default_strategy: "fastest", routes: [{ name: "default", models: ["gpt-4o-mini", "claude-3-haiku-20240307"], strategy: "fastest", timeout: "30s" }] },
    },
    tools: [
      { name: "multicall_stats", description: "Get multi-call statistics: wins per model, avg latency, cost comparison.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/multicall/stats" },
      { name: "multicall_routes", description: "List configured multi-call routes and their strategies.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/multicall/routes" },
      { name: "multicall_add_route", description: "Add a new multi-call route.", inputSchema: { type: "object", properties: { name: { type: "string" }, models: { type: "array", items: { type: "string" } }, strategy: { type: "string", enum: ["fastest", "cheapest", "longest", "shortest", "consensus"] } }, required: ["name", "models", "strategy"] }, apiPath: "/api/multicall/routes", method: "POST" },
      { name: "multicall_compare", description: "Send a prompt to all models and return all responses for comparison.", inputSchema: { type: "object", properties: { prompt: { type: "string" }, route: { type: "string", default: "default" } }, required: ["prompt"] }, apiPath: "/api/multicall/compare", method: "POST" },
      { name: "multicall_proxy_status", description: "Check if the MultiCall proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  streamsnap: {
    binary: "streamsnap",
    port: 5200,
    displayName: "StreamSnap",
    tagline: "Capture and replay every LLM stream",
    description: "SSE stream capture with zero latency overhead. Record TTFT, tokens/sec, and full responses. Replay captured streams for testing.",
    keywords: ["llm", "stream", "sse", "capture", "replay", "ttft", "latency", "proxy"],
    defaultConfig: {
      port: 5200, data_dir: "~/.stockyard", log_level: "info", product: "streamsnap",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      stream_snap: { enabled: true, retention: "72h", metrics: { ttft: true, tps: true }, replay: { enabled: true, auth: true } },
    },
    tools: [
      { name: "streamsnap_captures", description: "List recent stream captures with TTFT, tokens/sec, and metadata.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 }, model: { type: "string" } } }, apiPath: "/api/streams" },
      { name: "streamsnap_get", description: "Get full captured stream content by ID.", inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] }, apiPath: "/api/streams/detail" },
      { name: "streamsnap_metrics", description: "Get aggregated streaming metrics: avg TTFT, avg tokens/sec, completion rate.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/streams/metrics" },
      { name: "streamsnap_replay", description: "Replay a captured stream as if it were live.", inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] }, apiPath: "/api/streams/replay", method: "POST" },
      { name: "streamsnap_proxy_status", description: "Check if the StreamSnap proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  llmtap: {
    binary: "llmtap",
    port: 5300,
    displayName: "LLMTap",
    tagline: "Full-stack LLM analytics in one binary",
    description: "API analytics portal for LLM traffic. Latency percentiles (p50/p95/p99), error rates, cost breakdown by model, and volume tracking.",
    keywords: ["llm", "analytics", "monitoring", "latency", "dashboard", "observability", "proxy"],
    defaultConfig: {
      port: 5300, data_dir: "~/.stockyard", log_level: "info", product: "llmtap",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      llm_tap: { percentiles: [50, 95, 99], granularity: "hourly", retention: "30d" },
    },
    tools: [
      { name: "llmtap_overview", description: "Get analytics overview: total requests, latency percentiles, error rate, cost.", inputSchema: { type: "object", properties: { period: { type: "string", enum: ["1h", "24h", "7d", "30d"], default: "24h" } } }, apiPath: "/api/analytics/overview" },
      { name: "llmtap_latency", description: "Get detailed latency breakdown: p50, p95, p99 by model.", inputSchema: { type: "object", properties: { period: { type: "string", enum: ["1h", "24h", "7d", "30d"], default: "24h" }, model: { type: "string" } } }, apiPath: "/api/analytics/latency" },
      { name: "llmtap_errors", description: "Get error breakdown by type, model, and time window.", inputSchema: { type: "object", properties: { period: { type: "string", enum: ["1h", "24h", "7d", "30d"], default: "24h" } } }, apiPath: "/api/analytics/errors" },
      { name: "llmtap_costs", description: "Get cost analytics: spend per model, per endpoint, trends.", inputSchema: { type: "object", properties: { period: { type: "string", enum: ["1h", "24h", "7d", "30d"], default: "24h" }, group_by: { type: "string", enum: ["model", "endpoint", "hour"], default: "model" } } }, apiPath: "/api/analytics/costs" },
      { name: "llmtap_volume", description: "Get request volume over time for charting.", inputSchema: { type: "object", properties: { period: { type: "string", enum: ["1h", "24h", "7d", "30d"], default: "24h" }, granularity: { type: "string", enum: ["minute", "hourly", "daily"], default: "hourly" } } }, apiPath: "/api/analytics/volume" },
      { name: "llmtap_proxy_status", description: "Check if the LLMTap proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  contextpack: {
    binary: "contextpack",
    port: 5400,
    displayName: "ContextPack",
    tagline: "RAG without the vector database",
    description: "Rule-based context injection from local files. Keyword-matched chunks injected into prompts automatically. No embeddings, no vector DB.",
    keywords: ["llm", "rag", "context", "injection", "files", "knowledge", "retrieval", "proxy"],
    defaultConfig: {
      port: 5400, data_dir: "~/.stockyard", log_level: "info", product: "contextpack",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      context_pack: { sources: [{ name: "docs", type: "directory", path: "./docs", glob: "*.md" }], chunking: { size: 500, overlap: 50 }, injection: { position: "before_user", max_tokens: 2000, template: "Relevant context:\n{{chunks}}" } },
    },
    tools: [
      { name: "contextpack_sources", description: "List configured context sources and their chunk counts.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/context/sources" },
      { name: "contextpack_add_source", description: "Add a new context source.", inputSchema: { type: "object", properties: { name: { type: "string" }, type: { type: "string", enum: ["file", "directory", "inline"] }, path: { type: "string" }, content: { type: "string" }, glob: { type: "string", default: "*.md" } }, required: ["name", "type"] }, apiPath: "/api/context/sources", method: "POST" },
      { name: "contextpack_search", description: "Search context sources for relevant chunks. Preview what would be injected.", inputSchema: { type: "object", properties: { query: { type: "string" }, max_chunks: { type: "number", default: 5 } }, required: ["query"] }, apiPath: "/api/context/search", method: "POST" },
      { name: "contextpack_stats", description: "Get context injection statistics: total injections, avg chunks, token overhead.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/context/stats" },
      { name: "contextpack_reindex", description: "Re-index all context sources after adding new files.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/context/reindex", method: "POST" },
      { name: "contextpack_proxy_status", description: "Check if the ContextPack proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  retrypilot: {
    binary: "retrypilot",
    port: 5500,
    displayName: "RetryPilot",
    tagline: "Intelligent retries that actually work",
    description: "Smart retry engine with exponential backoff, circuit breakers, deadline awareness, and automatic model downgrade on failures.",
    keywords: ["llm", "retry", "circuit-breaker", "backoff", "resilience", "failover", "proxy"],
    defaultConfig: {
      port: 5500, data_dir: "~/.stockyard", log_level: "info", product: "retrypilot",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      retry_pilot: { max_retries: 3, backoff: "exponential", base_delay: "1s", jitter: "full", circuit_breaker: { threshold: 5, timeout: "30s", half_open_max: 2 }, deadline_aware: true, downgrade: { enabled: true, after_failures: 2, map: { "gpt-4o": "gpt-4o-mini" } }, budget: { max_per_minute: 20 } },
    },
    tools: [
      { name: "retrypilot_stats", description: "Get retry statistics: total retries, success rate, avg retries per request.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/retry/stats" },
      { name: "retrypilot_circuit_status", description: "Get circuit breaker status for each model: closed, open, or half-open.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/retry/circuits" },
      { name: "retrypilot_reset_circuit", description: "Manually reset a tripped circuit breaker.", inputSchema: { type: "object", properties: { model: { type: "string" } }, required: ["model"] }, apiPath: "/api/retry/circuits/reset", method: "POST" },
      { name: "retrypilot_set_config", description: "Update retry configuration dynamically.", inputSchema: { type: "object", properties: { max_retries: { type: "number" }, backoff: { type: "string", enum: ["exponential", "linear", "constant"] }, jitter: { type: "string", enum: ["full", "equal", "none"] } } }, apiPath: "/api/retry/config", method: "POST" },
      { name: "retrypilot_budget", description: "Get retry budget status: retries used this minute, remaining.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/retry/budget" },
      { name: "retrypilot_proxy_status", description: "Check if the RetryPilot proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  // ═══════════════════════════════════════════════════════════════════
  // PHASE 3 P1 — 12 PRODUCTS
  // ═══════════════════════════════════════════════════════════════════

  toxicfilter: {
    binary: "toxicfilter",
    port: 5600,
    displayName: "ToxicFilter",
    tagline: "Content moderation for LLM outputs",
    description: "Content moderation middleware for LLM responses. Block, redact, or flag harmful, hateful, or unsafe content before it reaches users.",
    keywords: ["llm", "moderation", "safety", "toxic", "content-filter", "harmful", "proxy"],
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

  compliancelog: {
    binary: "compliancelog",
    port: 5610,
    displayName: "ComplianceLog",
    tagline: "Immutable audit trail for every LLM call",
    description: "Tamper-proof audit logging for LLM interactions. Hash-chained entries, configurable retention, SOC2/HIPAA-ready export formats.",
    keywords: ["llm", "audit", "compliance", "soc2", "hipaa", "logging", "immutable", "proxy"],
    defaultConfig: {
      port: 5610, data_dir: "~/.stockyard", log_level: "info", product: "compliancelog",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      compliancelog: { hash_algorithm: "sha256", retention_days: 90, export_formats: ["json", "csv"], include_bodies: true },
    },
    tools: [
      { name: "compliancelog_stats", description: "Get audit log statistics: total entries, storage size, oldest/newest entry.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/compliance/stats" },
      { name: "compliancelog_search", description: "Search audit logs by date range, model, user, or project.", inputSchema: { type: "object", properties: { start: { type: "string" }, end: { type: "string" }, user: { type: "string" }, model: { type: "string" }, limit: { type: "number", default: 50 } } }, apiPath: "/api/compliance/search" },
      { name: "compliancelog_verify", description: "Verify hash chain integrity. Detects tampering.", inputSchema: { type: "object", properties: { start_id: { type: "number" }, end_id: { type: "number" } } }, apiPath: "/api/compliance/verify" },
      { name: "compliancelog_export", description: "Export audit logs in compliance format.", inputSchema: { type: "object", properties: { format: { type: "string", enum: ["json", "csv"], default: "json" }, start: { type: "string" }, end: { type: "string" } } }, apiPath: "/api/compliance/export" },
      { name: "compliancelog_proxy_status", description: "Check if the ComplianceLog proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  secretscan: {
    binary: "secretscan",
    port: 5620,
    displayName: "SecretScan",
    tagline: "Catch API keys leaking through LLM calls",
    description: "Detect and redact API keys, AWS credentials, tokens, and secrets in LLM requests and responses. TruffleHog-style pattern matching.",
    keywords: ["llm", "security", "secrets", "api-key", "credential", "leak", "redaction", "proxy"],
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

  tracelink: {
    binary: "tracelink",
    port: 5630,
    displayName: "TraceLink",
    tagline: "Distributed tracing for LLM chains",
    description: "Link related LLM calls into trace trees. Correlate multi-step agent workflows. OpenTelemetry-compatible with waterfall visualization.",
    keywords: ["llm", "tracing", "observability", "opentelemetry", "distributed", "agent", "correlation", "proxy"],
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

  alertpulse: {
    binary: "alertpulse",
    port: 5640,
    displayName: "AlertPulse",
    tagline: "PagerDuty for your LLM stack",
    description: "Configurable alerting for LLM infrastructure. Rules for error rates, latency, cost thresholds. Notify via Slack, Discord, PagerDuty, or webhooks.",
    keywords: ["llm", "alerting", "monitoring", "slack", "pagerduty", "webhook", "threshold", "proxy"],
    defaultConfig: {
      port: 5640, data_dir: "~/.stockyard", log_level: "info", product: "alertpulse",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      alertpulse: { rules: [{ name: "high-error-rate", metric: "error_rate", threshold: 0.1, window: "5m" }], channels: [{ type: "webhook", url: "${ALERT_WEBHOOK_URL}" }] },
    },
    tools: [
      { name: "alertpulse_rules", description: "List all alert rules with current status (firing/OK).", inputSchema: { type: "object", properties: {} }, apiPath: "/api/alerts/rules" },
      { name: "alertpulse_add_rule", description: "Add a new alert rule.", inputSchema: { type: "object", properties: { name: { type: "string" }, metric: { type: "string", enum: ["error_rate", "latency_p95", "cost_per_hour", "requests_per_minute"] }, threshold: { type: "number" }, window: { type: "string", default: "5m" } }, required: ["name", "metric", "threshold"] }, apiPath: "/api/alerts/rules", method: "POST" },
      { name: "alertpulse_history", description: "Get alert history: recent firings and resolutions.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/alerts/history" },
      { name: "alertpulse_test", description: "Fire a test alert to verify notification channels work.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/alerts/test", method: "POST" },
      { name: "alertpulse_proxy_status", description: "Check if the AlertPulse proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  chatmem: {
    binary: "chatmem",
    port: 5650,
    displayName: "ChatMem",
    tagline: "Persistent conversation memory without token bloat",
    description: "Conversation memory middleware. Sliding window, summarization, and importance-based strategies. Persist memory across sessions without eating context windows.",
    keywords: ["llm", "memory", "conversation", "context", "session", "chatbot", "persistence", "proxy"],
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

  mockllm: {
    binary: "mockllm",
    port: 5660,
    displayName: "MockLLM",
    tagline: "Deterministic LLM responses for testing",
    description: "Mock LLM server with canned responses for CI/CD pipelines. Define fixtures, simulate errors, control latency. Never hit real APIs in tests.",
    keywords: ["llm", "mock", "testing", "ci-cd", "fixtures", "deterministic", "simulation", "proxy"],
    defaultConfig: {
      port: 5660, data_dir: "~/.stockyard", log_level: "info", product: "mockllm",
      providers: {},
      mockllm: { mode: "fixture", fixtures: [{ match: ".*", response: "This is a mock response.", delay: "100ms" }] },
    },
    tools: [
      { name: "mockllm_fixtures", description: "List all configured mock fixtures with match patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/mock/fixtures" },
      { name: "mockllm_add_fixture", description: "Add a new mock fixture. Matched prompts return the canned response.", inputSchema: { type: "object", properties: { match: { type: "string" }, response: { type: "string" }, delay: { type: "string", default: "100ms" }, status: { type: "number", default: 200 } }, required: ["match", "response"] }, apiPath: "/api/mock/fixtures", method: "POST" },
      { name: "mockllm_remove_fixture", description: "Remove a mock fixture by ID.", inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] }, apiPath: "/api/mock/fixtures/remove", method: "POST" },
      { name: "mockllm_stats", description: "Get mock server stats: total requests, fixture match rate, error simulations.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/mock/stats" },
      { name: "mockllm_set_mode", description: "Switch mock mode: fixture, passthrough, or record.", inputSchema: { type: "object", properties: { mode: { type: "string", enum: ["fixture", "passthrough", "record"] } }, required: ["mode"] }, apiPath: "/api/mock/mode", method: "POST" },
      { name: "mockllm_proxy_status", description: "Check if the MockLLM proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  tenantwall: {
    binary: "tenantwall",
    port: 5670,
    displayName: "TenantWall",
    tagline: "Per-tenant isolation for multi-tenant LLM apps",
    description: "Tenant isolation middleware. Per-tenant rate limits, spend caps, model access, and cache isolation for multi-tenant AI products.",
    keywords: ["llm", "multi-tenant", "saas", "isolation", "rate-limit", "per-tenant", "b2b", "proxy"],
    defaultConfig: {
      port: 5670, data_dir: "~/.stockyard", log_level: "info", product: "tenantwall",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      tenantwall: { tenant_header: "X-Tenant-Id", default_limits: { requests_per_minute: 30, daily_spend: 10.0 } },
    },
    tools: [
      { name: "tenantwall_tenants", description: "List all known tenants with usage summary.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/tenants" },
      { name: "tenantwall_tenant_usage", description: "Get detailed usage for a specific tenant.", inputSchema: { type: "object", properties: { tenant_id: { type: "string" } }, required: ["tenant_id"] }, apiPath: "/api/tenants/usage" },
      { name: "tenantwall_set_limits", description: "Set rate limits and spend caps for a tenant.", inputSchema: { type: "object", properties: { tenant_id: { type: "string" }, requests_per_minute: { type: "number" }, daily_spend: { type: "number" } }, required: ["tenant_id"] }, apiPath: "/api/tenants/limits", method: "POST" },
      { name: "tenantwall_block", description: "Block a tenant from making LLM requests.", inputSchema: { type: "object", properties: { tenant_id: { type: "string" } }, required: ["tenant_id"] }, apiPath: "/api/tenants/block", method: "POST" },
      { name: "tenantwall_stats", description: "Get multi-tenant statistics: active tenants, total spend, top consumers.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/tenants/stats" },
      { name: "tenantwall_proxy_status", description: "Check if the TenantWall proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  idlekill: {
    binary: "idlekill",
    port: 5680,
    displayName: "IdleKill",
    tagline: "Kill runaway LLM requests before they drain your wallet",
    description: "Request watchdog middleware. Kill LLM requests exceeding time, token, or cost limits. Stop agent loops and runaway completions.",
    keywords: ["llm", "timeout", "watchdog", "runaway", "cost", "agent", "kill", "proxy"],
    defaultConfig: {
      port: 5680, data_dir: "~/.stockyard", log_level: "info", product: "idlekill",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      idlekill: { max_duration: "120s", max_tokens: 16000, max_cost: 1.0, action: "kill" },
    },
    tools: [
      { name: "idlekill_stats", description: "Get watchdog statistics: total monitored, killed, reasons for kills.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/idlekill/stats" },
      { name: "idlekill_active", description: "List currently active LLM requests being monitored.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/idlekill/active" },
      { name: "idlekill_set_limits", description: "Update kill thresholds.", inputSchema: { type: "object", properties: { max_duration: { type: "string" }, max_tokens: { type: "number" }, max_cost: { type: "number" } } }, apiPath: "/api/idlekill/limits", method: "POST" },
      { name: "idlekill_recent_kills", description: "List recently killed requests with reasons.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/idlekill/kills" },
      { name: "idlekill_proxy_status", description: "Check if the IdleKill proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  ipfence: {
    binary: "ipfence",
    port: 5690,
    displayName: "IPFence",
    tagline: "IP allowlisting for your LLM endpoints",
    description: "IP-level access control for LLM proxy endpoints. Allowlist, denylist, CIDR ranges. Block unauthorized access before any processing.",
    keywords: ["llm", "security", "ip", "allowlist", "firewall", "access-control", "cidr", "proxy"],
    defaultConfig: {
      port: 5690, data_dir: "~/.stockyard", log_level: "info", product: "ipfence",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      ipfence: { mode: "allowlist", action: "block", allowlist: ["127.0.0.1/8", "10.0.0.0/8", "192.168.0.0/16"] },
    },
    tools: [
      { name: "ipfence_stats", description: "Get access control statistics: requests checked, blocked, allowed, unique IPs.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/ipfence/stats" },
      { name: "ipfence_add_allow", description: "Add an IP or CIDR range to the allowlist.", inputSchema: { type: "object", properties: { ip: { type: "string" } }, required: ["ip"] }, apiPath: "/api/ipfence/allow", method: "POST" },
      { name: "ipfence_add_deny", description: "Add an IP or CIDR range to the denylist.", inputSchema: { type: "object", properties: { ip: { type: "string" } }, required: ["ip"] }, apiPath: "/api/ipfence/deny", method: "POST" },
      { name: "ipfence_check", description: "Check if a specific IP would be allowed or blocked.", inputSchema: { type: "object", properties: { ip: { type: "string" } }, required: ["ip"] }, apiPath: "/api/ipfence/check" },
      { name: "ipfence_recent", description: "List recent access events (allowed and blocked).", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/ipfence/recent" },
      { name: "ipfence_proxy_status", description: "Check if the IPFence proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  embedcache: {
    binary: "embedcache",
    port: 5700,
    displayName: "EmbedCache",
    tagline: "Never compute the same embedding twice",
    description: "Embedding response caching for /v1/embeddings. Content-hash deduplication, per-input splitting, 7-day TTL. Slash embedding costs for RAG pipelines.",
    keywords: ["llm", "embedding", "cache", "rag", "vector", "deduplication", "cost", "proxy"],
    defaultConfig: {
      port: 5700, data_dir: "~/.stockyard", log_level: "info", product: "embedcache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
      embedcache: { max_entries: 100000, ttl: "168h" },
    },
    tools: [
      { name: "embedcache_stats", description: "Get cache statistics: entries, hit rate, bytes saved, evictions.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/embedcache/stats" },
      { name: "embedcache_flush", description: "Clear the embedding cache.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/embedcache/flush", method: "POST" },
      { name: "embedcache_lookup", description: "Check if a specific text has a cached embedding.", inputSchema: { type: "object", properties: { text: { type: "string" }, model: { type: "string", default: "text-embedding-3-small" } }, required: ["text"] }, apiPath: "/api/embedcache/lookup", method: "POST" },
      { name: "embedcache_set_ttl", description: "Change cache TTL for new entries.", inputSchema: { type: "object", properties: { ttl: { type: "string" } }, required: ["ttl"] }, apiPath: "/api/embedcache/ttl", method: "POST" },
      { name: "embedcache_proxy_status", description: "Check if the EmbedCache proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  anthrofit: {
    binary: "anthrofit",
    port: 5710,
    displayName: "AnthroFit",
    tagline: "Use Claude with OpenAI SDKs",
    description: "Deep Anthropic compatibility layer. System prompt consolidation, max_tokens injection, tool schema translation, streaming normalization.",
    keywords: ["llm", "anthropic", "claude", "openai", "compatibility", "adapter", "translation", "proxy"],
    defaultConfig: {
      port: 5710, data_dir: "~/.stockyard", log_level: "info", product: "anthrofit",
      providers: { anthropic: { api_key: "${ANTHROPIC_API_KEY}", base_url: "https://api.anthropic.com" } },
      anthrofit: { system_prompt_mode: "auto", tool_translation: true, stream_normalize: true, max_tokens_default: 4096 },
    },
    tools: [
      { name: "anthrofit_stats", description: "Get translation statistics: requests processed, system prompts fixed, tools translated.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/anthrofit/stats" },
      { name: "anthrofit_test", description: "Test OpenAI to Anthropic translation on a request without sending.", inputSchema: { type: "object", properties: { model: { type: "string", default: "claude-sonnet-4-20250514" }, messages: { type: "array", items: { type: "object" } } }, required: ["messages"] }, apiPath: "/api/anthrofit/test", method: "POST" },
      { name: "anthrofit_set_mode", description: "Change system prompt handling mode.", inputSchema: { type: "object", properties: { mode: { type: "string", enum: ["auto", "separate", "merge"] } }, required: ["mode"] }, apiPath: "/api/anthrofit/mode", method: "POST" },
      { name: "anthrofit_config", description: "Get current AnthroFit configuration.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/anthrofit/config" },
      { name: "anthrofit_proxy_status", description: "Check if the AnthroFit proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  // ═══════════════════════════════════════════════════════════════════
  // UNIFIED SUITE
  // ═══════════════════════════════════════════════════════════════════

  stockyard: {
    binary: "stockyard",
    port: 4000,
    displayName: "Stockyard",
    tagline: "The complete LLM infrastructure suite",
    description: "All 125 Stockyard products in one binary. Cost control, caching, validation, routing, security, analytics, moderation, compliance, and more.",
    keywords: ["llm", "proxy", "infrastructure", "suite", "openai", "anthropic", "cost", "cache", "routing", "security"],
    defaultConfig: {
      port: 4000, data_dir: "~/.stockyard", log_level: "info", product: "stockyard",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "stockyard_status", description: "Get full suite status: all enabled features, health, and summary stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/status" },
      { name: "stockyard_spend", description: "Get current LLM spending across all projects.", inputSchema: { type: "object", properties: { project: { type: "string", default: "default" } } }, apiPath: "/api/spend" },
      { name: "stockyard_cache_stats", description: "Get cache hit/miss statistics and savings.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/cache/stats" },
      { name: "stockyard_providers", description: "Get health and routing status of all providers.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/providers/status" },
      { name: "stockyard_analytics", description: "Get analytics overview: latency, error rates, costs, volume.", inputSchema: { type: "object", properties: { period: { type: "string", enum: ["1h", "24h", "7d", "30d"], default: "24h" } } }, apiPath: "/api/analytics/overview" },
      { name: "stockyard_logs", description: "List recent LLM request logs.", inputSchema: { type: "object", properties: { limit: { type: "number", default: 20 } } }, apiPath: "/api/logs" },
      { name: "stockyard_proxy_status", description: "Check if the Stockyard suite is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
};

// Merge Phase 3 P2/P3 + Phase 4 expansion products
const { EXPANSION_PRODUCTS } = require("./products_expansion");
Object.assign(PRODUCTS, EXPANSION_PRODUCTS);

module.exports = { PRODUCTS };
