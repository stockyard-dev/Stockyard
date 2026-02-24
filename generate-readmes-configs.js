#!/usr/bin/env node
/**
 * Generate README.md + example YAML config for all 125 Stockyard products.
 * Skips files that already exist.
 */

const fs = require("fs");
const path = require("path");

const CMD_DIR = path.join(__dirname, "cmd");
const CFG_DIR = path.join(__dirname, "configs");

let readmes = 0, configs = 0, skipR = 0, skipC = 0;

// [binary, displayName, port, tagline, description, features[], configYAML]
const PRODUCTS = [

// ─── ORIGINAL 7 ────────────────────────────────────────────────
["costcap", "CostCap", 4100,
  "Never get a surprise LLM bill again.",
  "CostCap enforces hard and soft spending caps on LLM API calls. Set daily or monthly budgets per project, model, or API key. Get webhook alerts at configurable thresholds.",
  ["Hard spending caps with 429 responses", "Per-project and per-model budgets", "Alert webhooks at configurable thresholds", "Live dashboard with real-time spend", "Per-model cost breakdown", "Zero code changes — just change base URL"],
  `port: 4100
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
projects:
  default:
    caps:
      daily: 5.00
      monthly: 50.00
    alerts:
      webhook: ""
      thresholds: [0.5, 0.8, 1.0]`],

["llmcache", "CacheLayer", 4200,
  "Stop paying for the same response twice.",
  "CacheLayer caches LLM responses using exact prompt matching with configurable TTL. Cached responses return instantly, saving tokens and reducing latency.",
  ["Exact-match response caching", "Configurable TTL per model", "Cache hit/miss dashboard", "Streaming response replay", "Cache invalidation API", "SQLite storage — no Redis needed"],
  `port: 4200
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
cache:
  enabled: true
  ttl: 3600         # 1 hour default
  max_entries: 10000
  match_strategy: exact  # exact | normalized`],

["jsonguard", "StructuredShield", 4300,
  "LLMs that actually return valid JSON.",
  "StructuredShield validates LLM responses against JSON schemas and auto-retries on parse failure. Stop writing try/catch around every JSON.parse.",
  ["JSON schema validation on responses", "Auto-retry on parse failure (up to 3x)", "Schema registry with versioning", "Streaming-aware validation", "Detailed error diagnostics", "Works with any model that attempts JSON"],
  `port: 4300
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
validation:
  enabled: true
  max_retries: 3
  schemas:
    default:
      type: object
      required: [answer]
      properties:
        answer:
          type: string`],

["routefall", "FallbackRouter", 4400,
  "When OpenAI goes down, your app doesn't.",
  "FallbackRouter provides automatic provider failover with circuit breaker patterns and health checks. Define a priority chain of providers and models.",
  ["Automatic provider failover", "Circuit breaker with configurable thresholds", "Health check endpoints", "Latency-aware routing", "Per-provider error tracking", "Zero-downtime provider switches"],
  `port: 4400
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
    priority: 1
  anthropic:
    api_key: \${ANTHROPIC_API_KEY}
    priority: 2
  groq:
    api_key: \${GROQ_API_KEY}
    priority: 3
fallback:
  circuit_breaker:
    threshold: 5
    window: 60s
    recovery: 30s`],

["rateshield", "RateShield", 4500,
  "Rate limiting that actually makes sense for LLMs.",
  "RateShield provides token bucket rate limiting per API key, IP, or custom identifier. Protect your LLM budget from runaway clients.",
  ["Token bucket rate limiting", "Per-key and per-IP limits", "Burst allowance configuration", "429 responses with Retry-After headers", "Dashboard with limit utilization", "Request queuing option"],
  `port: 4500
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
rate_limit:
  default:
    requests_per_minute: 60
    tokens_per_minute: 100000
    burst: 10`],

["promptreplay", "PromptReplay", 4600,
  "Record everything. Replay anything.",
  "PromptReplay logs every LLM request and response with full metadata. Search, filter, and replay past requests for debugging and analysis.",
  ["Full request/response logging", "Search and filter by model, status, cost", "One-click request replay", "Export as JSONL for training data", "Configurable retention policies", "Dashboard with request explorer"],
  `port: 4600
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
logging:
  store_bodies: true
  retention_days: 30
  max_storage_mb: 500`],

["stockyard", "Stockyard Suite", 4000,
  "Every LLM tool you need. One binary.",
  "The full Stockyard suite — all 125 products in a single binary. Cost caps, caching, rate limiting, failover, logging, and 120 more middleware tools.",
  ["All 125 products in one binary", "Single YAML config for everything", "Unified dashboard", "63-step middleware chain", "17+ provider adapters", "6MB static binary, zero dependencies"],
  `port: 4000
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
  anthropic:
    api_key: \${ANTHROPIC_API_KEY}

# Enable products by adding their config sections:
costcap:
  enabled: true
  projects:
    default:
      caps: { daily: 10.00 }

cache:
  enabled: true
  ttl: 3600

rateshield:
  enabled: true
  default:
    requests_per_minute: 60`],

// ─── PHASE 1 ──────────────────────────────────────────────────
["keypool", "KeyPool", 4700,
  "Pool API keys. Rotate on rate limits.",
  "KeyPool manages multiple API keys per provider with automatic rotation strategies. When one key hits rate limits, seamlessly rotate to the next.",
  ["Multiple keys per provider", "Round-robin, least-used, and random strategies", "Auto-rotate on 429 responses", "Per-key usage tracking", "Key health monitoring", "Dashboard with key utilization"],
  `port: 4700
providers:
  openai:
    keys:
      - \${OPENAI_API_KEY_1}
      - \${OPENAI_API_KEY_2}
      - \${OPENAI_API_KEY_3}
    strategy: round-robin  # round-robin | least-used | random
    rotate_on_429: true`],

["promptguard", "PromptGuard", 4710,
  "PII never reaches the model.",
  "PromptGuard detects and redacts personally identifiable information from prompts before they reach the LLM. Also detects prompt injection attempts.",
  ["Regex-based PII detection and redaction", "Email, phone, SSN, credit card patterns", "Prompt injection detection", "Redact or block modes", "Custom pattern definitions", "Dashboard with detection stats"],
  `port: 4710
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
pii:
  mode: redact  # redact | block
  patterns:
    - email
    - phone
    - ssn
    - credit_card
  custom_patterns:
    - name: api_key
      regex: "sk-[a-zA-Z0-9]{20,}"
injection:
  enabled: true
  sensitivity: medium  # low | medium | high`],

["modelswitch", "ModelSwitch", 4720,
  "Route requests to the right model automatically.",
  "ModelSwitch routes LLM requests to different models based on token count, prompt patterns, custom headers, or cost rules. A/B test models with traffic splits.",
  ["Rule-based model routing", "Route by token count, pattern, or header", "A/B testing with traffic splits", "Cost tracking per route", "Tiered model chains", "Dashboard with routing analytics"],
  `port: 4720
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
routing:
  rules:
    - match: { max_tokens: 100 }
      model: gpt-4o-mini
    - match: { pattern: "code|debug|refactor" }
      model: gpt-4o
    - match: { header: "x-priority: high" }
      model: gpt-4o
  default_model: gpt-4o-mini`],

["evalgate", "EvalGate", 4730,
  "Score every response. Retry the bad ones.",
  "EvalGate runs quality validators on LLM responses and auto-retries when quality is below threshold. Validators include JSON parsing, length checks, regex matching, and custom expressions.",
  ["Response quality scoring", "Auto-retry on low quality", "JSON parse validation", "Min/max length checks", "Regex pattern matching", "Configurable retry budget"],
  `port: 4730
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
eval:
  validators:
    - type: json_parse
    - type: min_length
      value: 50
    - type: regex_match
      pattern: "\\\\b(answer|result)\\\\b"
  retry:
    max_attempts: 3
    min_score: 0.7`],

["usagepulse", "UsagePulse", 4740,
  "Know exactly who's using what.",
  "UsagePulse provides per-user, per-feature, and per-team token metering. Track usage across dimensions, set spend caps, and export billing data.",
  ["Multi-dimensional usage metering", "Per-user, feature, and team tracking", "Spend caps per dimension", "CSV/JSON billing export", "Webhook notifications", "Dashboard with usage breakdowns"],
  `port: 4740
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
metering:
  dimensions:
    - header: x-user-id
    - header: x-feature
    - header: x-team
  caps:
    per_user_daily: 10000  # tokens
  export:
    format: csv
    schedule: daily`],

// ─── PHASE 2 ──────────────────────────────────────────────────
["promptpad", "PromptPad", 4800,
  "Version control for prompts.",
  "PromptPad manages versioned prompt templates with A/B testing. Change prompts without redeploying code.",
  ["Versioned prompt templates", "A/B testing across versions", "Hot-reload without restarts", "Template variables with defaults", "Performance tracking per version", "Dashboard with version history"],
  `port: 4800
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
templates:
  greeting:
    active: v2
    versions:
      v1: "You are a helpful assistant."
      v2: "You are a concise, technical assistant. Be direct."`],

["tokentrim", "TokenTrim", 4900,
  "Fit more into your context window.",
  "TokenTrim optimizes context window usage with smart truncation strategies. Prioritize recent messages, trim system prompts, or use custom strategies.",
  ["Smart context window truncation", "Prioritize recent messages", "System prompt compression", "Configurable strategies per model", "Token count visibility", "Works with any context window size"],
  `port: 4900
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
truncation:
  strategy: recent_first  # recent_first | oldest_first | smart
  reserve_system: 500     # tokens reserved for system prompt
  reserve_response: 1000  # tokens reserved for response
  target_ratio: 0.8       # fill to 80% of context window`],

["batchqueue", "BatchQueue", 5000,
  "Queue it up. Process it later.",
  "BatchQueue provides async request queuing with configurable concurrency control. Submit jobs, get results via polling or webhook.",
  ["Async request queue", "Configurable concurrency limits", "Job status polling API", "Webhook on completion", "Priority levels", "Dashboard with queue depth"],
  `port: 5000
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
queue:
  max_concurrent: 5
  max_queue_depth: 1000
  timeout: 120s
  webhook_on_complete: ""`],

["multicall", "MultiCall", 5100,
  "Ask multiple models. Compare answers.",
  "MultiCall sends the same prompt to multiple models simultaneously and returns all responses for comparison or consensus voting.",
  ["Multi-model parallel requests", "Consensus voting modes", "Side-by-side comparison", "Latency and cost per model", "Configurable timeout per model", "Dashboard with comparison history"],
  `port: 5100
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
  anthropic:
    api_key: \${ANTHROPIC_API_KEY}
multicall:
  models:
    - openai/gpt-4o
    - anthropic/claude-sonnet-4-20250514
  mode: all       # all | fastest | consensus
  timeout: 30s`],

["streamsnap", "StreamSnap", 5200,
  "Capture and replay SSE streams.",
  "StreamSnap records streaming LLM responses with original chunk timing. Replay streams for debugging or cache hits that feel natural.",
  ["SSE stream capture with timing", "Faithful replay with original delays", "TTFT (time to first token) metrics", "Stream comparison tools", "Export captured streams", "Dashboard with stream explorer"],
  `port: 5200
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
streamsnap:
  capture: true
  store_timing: true
  replay_mode: realistic  # realistic | instant
  retention_days: 7`],

["llmtap", "LLMTap", 5300,
  "Full analytics for your LLM traffic.",
  "LLMTap provides a complete analytics portal for LLM API traffic. Track p50/p95/p99 latency, cost trends, error rates, and token usage across all models.",
  ["p50/p95/p99 latency tracking", "Cost trend analytics", "Error rate monitoring", "Token usage breakdowns", "Per-model and per-endpoint stats", "Interactive dashboard with drill-down"],
  `port: 5300
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
analytics:
  enabled: true
  retention_days: 90
  aggregation_interval: 1m`],

["contextpack", "ContextPack", 5400,
  "Poor man's RAG. Inject context from anywhere.",
  "ContextPack injects context from files, SQLite databases, or URLs into LLM prompts. Lightweight RAG without a vector database.",
  ["Inject context from files, SQLite, or URLs", "Automatic chunking and relevance scoring", "Template-based context injection", "Configurable context budget", "Source attribution in responses", "No vector database needed"],
  `port: 5400
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
context:
  sources:
    - type: file
      path: ./docs/
      glob: "*.md"
    - type: url
      urls:
        - https://docs.example.com/api
  max_tokens: 2000
  strategy: relevance  # relevance | all | random`],

["retrypilot", "RetryPilot", 5500,
  "Smart retries that don't make things worse.",
  "RetryPilot provides intelligent retry logic with exponential backoff, jitter, circuit breakers, and automatic model downgrade on persistent failures.",
  ["Exponential backoff with jitter", "Circuit breaker pattern", "Model downgrade on persistent failure", "Per-error-type retry strategies", "Max retry budget (cost and count)", "Dashboard with retry analytics"],
  `port: 5500
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
retry:
  max_attempts: 3
  backoff: exponential
  jitter: true
  circuit_breaker:
    threshold: 5
    window: 60s
  downgrade_chain:
    - gpt-4o
    - gpt-4o-mini`],

// ─── PHASE 3 P1 ──────────────────────────────────────────────
["toxicfilter", "ToxicFilter", 5600,
  "Keep harmful content out of your app.",
  "ToxicFilter scans LLM outputs for harmful, toxic, or inappropriate content. Block, redact, or flag based on configurable rule sets.",
  ["Output content moderation", "Keyword and regex rule engine", "Block, redact, or flag modes", "Category-based filtering", "Custom blocklists", "Dashboard with moderation stats"],
  `port: 5600
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
toxicfilter:
  mode: block    # block | redact | flag
  categories:
    - hate_speech
    - violence
    - sexual_content
    - self_harm
  custom_blocklist:
    - pattern1
    - pattern2`],

["compliancelog", "ComplianceLog", 5610,
  "Immutable audit trail for every LLM call.",
  "ComplianceLog creates tamper-evident, append-only logs of all LLM interactions. Hash-chained entries with configurable retention for SOC2/HIPAA compliance.",
  ["Append-only audit logs", "Hash-chain tamper detection", "Configurable retention periods", "Compliance export formats", "Per-field encryption option", "Dashboard with audit explorer"],
  `port: 5610
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
compliance:
  enabled: true
  hash_chain: true
  retention_days: 365
  export_format: jsonl  # jsonl | csv
  encrypt_bodies: false`],

["secretscan", "SecretScan", 5620,
  "Catch API keys leaking through your LLM.",
  "SecretScan detects API keys, passwords, and secrets in both requests and responses. Blocks or redacts before they reach the model or your users.",
  ["Bidirectional secret scanning", "AWS, GCP, GitHub, Stripe key patterns", "Block or redact modes", "Custom pattern definitions", "Higher severity than PII", "Dashboard with detection log"],
  `port: 5620
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
secretscan:
  mode: redact  # block | redact | alert
  patterns:
    - aws_key
    - github_token
    - stripe_key
    - generic_api_key
    - private_key`],

["tracelink", "TraceLink", 5630,
  "Distributed tracing for LLM chains.",
  "TraceLink propagates trace IDs across multi-step LLM calls. Link parent-child requests into trees for debugging agent workflows.",
  ["X-Trace-ID propagation", "Parent-child request linking", "Waterfall visualization", "OpenTelemetry compatible", "Per-trace cost and latency", "Dashboard with trace explorer"],
  `port: 5630
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
tracing:
  enabled: true
  header: X-Trace-ID
  auto_generate: true
  otlp_export: ""  # optional OTLP endpoint`],

["alertpulse", "AlertPulse", 5640,
  "PagerDuty for your LLM stack.",
  "AlertPulse monitors error rates, latency, and costs with configurable alert rules. Fire webhooks to Slack, Discord, PagerDuty, or email.",
  ["Configurable alert rules", "Error rate, latency, cost thresholds", "Slack/Discord/PagerDuty webhooks", "Cooldown periods", "Sliding window metrics", "Dashboard with alert history"],
  `port: 5640
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
alerts:
  rules:
    - name: high_errors
      metric: error_rate
      threshold: 25
      webhook: \${ALERT_WEBHOOK}
    - name: cost_spike
      metric: cost_per_min
      threshold: 1.0
      webhook: \${ALERT_WEBHOOK}
  cooldown: 5m`],

["chatmem", "ChatMem", 5650,
  "Persistent memory without eating your context window.",
  "ChatMem manages conversation memory with smart strategies. Sliding window, summarization, and importance-based retention keep context relevant without burning tokens.",
  ["Session-based conversation memory", "Sliding window strategy", "Auto-summarization of old messages", "Importance-based retention", "Configurable memory budget", "Dashboard with session explorer"],
  `port: 5650
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
memory:
  strategy: sliding_window  # sliding_window | summarize | importance
  max_tokens: 4000
  summarize_after: 20  # messages
  session_header: X-Session-ID`],

["mockllm", "MockLLM", 5660,
  "Deterministic LLM responses for testing.",
  "MockLLM provides a fake LLM server with fixture-based responses. Perfect for CI/CD — no API keys, no costs, deterministic behavior.",
  ["Fixture-based responses", "Prompt pattern matching", "Regex and exact match modes", "Configurable latency simulation", "Error simulation", "Zero API costs in CI"],
  `port: 5660
providers: {}  # No real providers needed
mock:
  fixtures:
    - match: "hello"
      response: "Hi! How can I help you?"
    - match: ".*json.*"
      type: regex
      response: '{"answer": "mock response"}'
  default_response: "This is a mock response."
  latency_ms: 100  # Simulate real latency`],

["tenantwall", "TenantWall", 5670,
  "Per-tenant isolation for multi-tenant apps.",
  "TenantWall provides per-tenant rate limits, spend caps, model access controls, and cache isolation. Build multi-tenant AI SaaS without custom infrastructure.",
  ["Per-tenant rate limits", "Per-tenant spend caps", "Model access controls per tenant", "Cache isolation", "Tenant ID via header or key prefix", "Dashboard with per-tenant metrics"],
  `port: 5670
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
tenants:
  default:
    rate_limit: 60/min
    daily_cap: 5.00
    allowed_models: [gpt-4o-mini]
  premium:
    rate_limit: 300/min
    daily_cap: 50.00
    allowed_models: [gpt-4o, gpt-4o-mini]
tenant_header: X-Tenant-ID`],

["idlekill", "IdleKill", 5680,
  "Kill runaway requests before they kill your budget.",
  "IdleKill monitors individual request duration, token count, and cost in real-time. Terminates requests that exceed configurable thresholds.",
  ["Real-time request cost monitoring", "Max duration per request", "Max tokens per request", "Max cost per request", "Streaming-aware termination", "Webhook alerts on kills"],
  `port: 5680
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
idlekill:
  max_duration: 60s
  max_tokens: 10000
  max_cost: 0.50
  alert_webhook: ""`],

["ipfence", "IPFence", 5690,
  "IP allowlisting for your LLM endpoints.",
  "IPFence restricts access to your proxy by IP address, CIDR range, or country code. Prevent unauthorized access and bill theft.",
  ["IP allowlist and denylist", "CIDR range support", "Country-based geofencing", "Automatic GeoIP lookup", "Fail-open or fail-closed modes", "Dashboard with blocked request log"],
  `port: 5690
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
ipfence:
  mode: allowlist  # allowlist | denylist
  allow:
    - 10.0.0.0/8
    - 192.168.0.0/16
    - 203.0.113.50
  deny: []
  block_countries: []  # e.g. [CN, RU]`],

["embedcache", "EmbedCache", 5700,
  "Never compute the same embedding twice.",
  "EmbedCache caches embedding API responses using content hashing. Get 100% cache hit rate on re-indexed documents.",
  ["Content-hash based embedding cache", "100% hit rate on re-indexing", "Works with /v1/embeddings endpoint", "Tracks cache savings", "SQLite storage", "Dashboard with hit rate metrics"],
  `port: 5700
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
embedcache:
  enabled: true
  ttl: 0          # 0 = never expire (embeddings are deterministic)
  max_entries: 100000`],

["anthrofit", "AnthroFit", 5710,
  "Use Claude with OpenAI SDKs.",
  "AnthroFit provides deep translation between OpenAI and Anthropic API formats. System messages, tool schemas, streaming format, and response structure all handled.",
  ["OpenAI-to-Anthropic API translation", "System message conversion", "Tool/function call schema mapping", "Streaming format translation", "Response structure normalization", "Drop-in Anthropic support"],
  `port: 5710
providers:
  anthropic:
    api_key: \${ANTHROPIC_API_KEY}
anthrofit:
  enabled: true
  default_model: claude-sonnet-4-20250514
  model_map:
    gpt-4o: claude-sonnet-4-20250514
    gpt-4o-mini: claude-haiku-4-5-20251001`],

// ─── PHASE 3 P2 ──────────────────────────────────────────────
["agentguard", "AgentGuard", 5720, "Safety rails for autonomous agents.",
  "AgentGuard tracks agent sessions and enforces per-session limits on calls, cost, duration, and allowed tools.",
  ["Per-session call limits", "Per-session cost caps", "Max session duration", "Allowed tool restrictions", "Kill session on breach", "Dashboard with session monitor"],
  `port: 5720
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
agentguard:
  session_header: X-Session-ID
  max_calls: 50
  max_cost: 5.00
  max_duration: 300s
  allowed_tools: []  # empty = all allowed`],

["codefence", "CodeFence", 5730, "Validate LLM-generated code before it runs.",
  "CodeFence detects code blocks in LLM output and runs safety checks: syntax validation, forbidden pattern detection, and complexity scoring.",
  ["Code block detection", "Syntax validation", "Forbidden pattern matching", "Complexity scoring", "Language-aware checks", "Block or warn modes"],
  `port: 5730
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
codefence:
  mode: warn  # block | warn
  forbidden_patterns:
    - "rm -rf"
    - "eval("
    - "exec("
    - "DROP TABLE"
  max_complexity: 50`],

["hallucicheck", "HalluciCheck", 5740, "Catch hallucinations before your users do.",
  "HalluciCheck extracts URLs, emails, and citations from LLM responses and validates them. Cross-references against provided context sources.",
  ["URL validation in responses", "Email format checking", "Citation cross-referencing", "Confidence scoring", "Flag or retry modes", "Dashboard with hallucination rate"],
  `port: 5740
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
hallucicheck:
  check_urls: true
  check_emails: true
  check_citations: true
  mode: flag  # flag | retry | block
  confidence_threshold: 0.7`],

["tierdrop", "TierDrop", 5750, "Auto-downgrade models when burning cash.",
  "TierDrop automatically switches to cheaper models as spending approaches budget limits. Graceful degradation instead of hard blocks.",
  ["Cost-aware model degradation", "Integrates with CostCap spend data", "Configurable tier thresholds", "Transparent to callers", "Model quality chain", "Dashboard with tier usage"],
  `port: 5750
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
tierdrop:
  tiers:
    - model: gpt-4o
      max_spend_percent: 70
    - model: gpt-4o-mini
      max_spend_percent: 90
    - model: gpt-3.5-turbo
      max_spend_percent: 100
  daily_budget: 10.00`],

["driftwatch", "DriftWatch", 5760, "Detect model behavior changes before users notice.",
  "DriftWatch runs baseline prompts periodically and compares output characteristics over time. Alerts when model behavior drifts.",
  ["Periodic baseline testing", "Output characteristic tracking", "Statistical drift detection", "Alert on behavior change", "Historical trend charts", "Custom baseline prompts"],
  `port: 5760
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
driftwatch:
  schedule: "0 */6 * * *"  # every 6 hours
  baselines:
    - prompt: "What is 2+2?"
      expect_contains: "4"
    - prompt: "Write a haiku about coding."
      expect_min_length: 30
  alert_webhook: ""`],

["feedbackloop", "FeedbackLoop", 5770, "Collect user feedback. Close the loop.",
  "FeedbackLoop captures user ratings linked to specific LLM requests. Track which prompts produce bad responses and export for improvement.",
  ["Per-request feedback capture", "Thumbs up/down and ratings", "Link feedback to request IDs", "Worst-performing prompt reports", "Export for fine-tuning", "Dashboard with feedback trends"],
  `port: 5770
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
feedback:
  enabled: true
  endpoint: /api/feedback  # POST {request_id, rating, comment}
  retention_days: 90`],

["abrouter", "ABRouter", 5780, "A/B test any LLM variable with statistical rigor.",
  "ABRouter runs controlled experiments across models, temperatures, prompts, or providers with proper statistical significance testing.",
  ["Multi-variable A/B testing", "Statistical significance testing", "Configurable traffic splits", "Auto-promote winners", "Cost and quality per variant", "Dashboard with experiment results"],
  `port: 5780
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
experiments:
  model_test:
    variable: model
    variants:
      control: gpt-4o
      challenger: gpt-4o-mini
    split: [50, 50]
    metric: quality_score
    min_samples: 100`],

["guardrail", "GuardRail", 5790, "Keep your LLM on-script.",
  "GuardRail enforces topic boundaries on LLM output. Prevent your customer support bot from giving medical advice or your code assistant from writing poetry.",
  ["Topic boundary enforcement", "Allow/deny topic categories", "Output classification", "Custom category definitions", "Fallback messages for off-topic", "Dashboard with boundary violations"],
  `port: 5790
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
guardrail:
  allowed_topics:
    - customer_support
    - product_info
    - billing
  denied_topics:
    - medical_advice
    - legal_advice
    - financial_advice
  fallback_message: "I can only help with product-related questions."`],

["geminishim", "GeminiShim", 5800, "Tame Gemini's quirks.",
  "GeminiShim handles Gemini-specific issues: random safety filter blocks, inconsistent JSON mode, and multimodal format differences.",
  ["Auto-retry on safety filter blocks", "JSON mode normalization", "Multimodal format translation", "Token count normalization", "Gemini-specific error handling", "Drop-in Gemini support"],
  `port: 5800
providers:
  gemini:
    api_key: \${GEMINI_API_KEY}
geminishim:
  retry_safety_blocks: true
  max_safety_retries: 3
  normalize_json: true
  model_map:
    gpt-4o: gemini-1.5-pro
    gpt-4o-mini: gemini-1.5-flash`],

["localsync", "LocalSync", 5810, "Blend local and cloud models seamlessly.",
  "LocalSync health-checks local model endpoints and routes to them when available, failing over to cloud when they're down.",
  ["Local endpoint health checking", "Auto-failover to cloud", "Cost savings tracking", "Latency comparison", "Configurable health intervals", "Dashboard with local vs cloud usage"],
  `port: 5810
providers:
  local:
    base_url: http://localhost:11434/v1  # Ollama
    api_key: not-needed
    health_check: true
  openai:
    api_key: \${OPENAI_API_KEY}
localsync:
  prefer_local: true
  health_interval: 10s
  fallback_provider: openai`],

["devproxy", "DevProxy", 5820, "Charles Proxy for LLM APIs.",
  "DevProxy provides an interactive debugging dashboard for LLM traffic. Inspect, pause, edit, and replay requests in real-time.",
  ["Live WebSocket request inspector", "Pause/resume request flow", "Edit requests before forwarding", "Breakpoints on patterns", "Request/response diff view", "Interactive debugging dashboard"],
  `port: 5820
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
devproxy:
  capture: true
  breakpoints:
    - pattern: "DELETE"
    - header: "x-debug: true"
  websocket_port: 5821`],

["promptslim", "PromptSlim", 5830, "Compress prompts by 40-70% without losing meaning.",
  "PromptSlim removes filler words, deduplicates instructions, and compresses whitespace in prompts. Configurable aggressiveness levels.",
  ["Remove articles and filler words", "Deduplicate repeated instructions", "Compress whitespace", "Configurable aggressiveness", "Before/after token comparison", "Dashboard with savings stats"],
  `port: 5830
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
promptslim:
  aggressiveness: medium  # low | medium | high
  preserve_code_blocks: true
  preserve_urls: true`],

["promptlint", "PromptLint", 5840, "Catch prompt anti-patterns before they cost you.",
  "PromptLint performs static analysis on prompts: detects redundancy, conflicting instructions, injection vulnerabilities, and missing format specs.",
  ["Redundancy detection", "Conflict detection", "Injection vulnerability scanning", "Missing format spec warnings", "Prompt quality scoring", "CLI and middleware modes"],
  `port: 5840
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
promptlint:
  mode: warn  # warn | block
  checks:
    - redundancy
    - conflicts
    - injection_risk
    - missing_format
  min_score: 0.5`],

["approvalgate", "ApprovalGate", 5850, "Human approval for prompt changes.",
  "ApprovalGate adds an approval workflow to PromptPad. Prompt changes go into pending state until an approver accepts or rejects.",
  ["Pending state for prompt changes", "Approver notification", "Approve/reject via dashboard", "Full audit trail", "Role-based approvers", "Integration with PromptPad"],
  `port: 5850
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
approval:
  enabled: true
  approvers:
    - admin@example.com
  notify_webhook: ""
  auto_approve_after: 0  # 0 = never auto-approve`],

["outputcap", "OutputCap", 5860, "Stop paying for responses you don't need.",
  "OutputCap monitors token count in streaming responses and cuts at natural sentence boundaries. Ask for one word, don't pay for an essay.",
  ["Natural boundary detection", "Sentence-aware truncation", "Token budget per request", "Streaming-aware cutting", "Cost savings tracking", "Configurable per model"],
  `port: 5860
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
outputcap:
  max_tokens: 500
  cut_at: sentence  # sentence | paragraph | word
  warn_header: true  # Add X-Output-Capped header`],

["agegate", "AgeGate", 5870, "Child safety middleware for LLM apps.",
  "AgeGate enforces age-appropriate content filtering. Configure age tiers, inject appropriate system prompts, and filter adult content.",
  ["Age tier configuration", "Age-appropriate system prompt injection", "Adult content filtering", "Violence and self-harm filtering", "COPPA/KOSA compliance helpers", "Dashboard with filter stats"],
  `port: 5870
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
agegate:
  default_tier: adult
  tiers:
    child:    { max_age: 12, filter: strict }
    teen:     { max_age: 17, filter: moderate }
    adult:    { max_age: 999, filter: none }
  age_header: X-User-Age`],

["voicebridge", "VoiceBridge", 5880, "LLM output optimized for voice.",
  "VoiceBridge strips markdown, converts lists to prose, removes code blocks, and enforces max length for TTS-friendly output.",
  ["Strip markdown from output", "Convert lists to natural prose", "Remove code blocks", "Max length enforcement", "TTFB tracking for voice latency", "Configurable output style"],
  `port: 5880
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
voicebridge:
  strip_markdown: true
  convert_lists: true
  remove_code: true
  max_length: 300  # characters
  style: conversational`],

["imageproxy", "ImageProxy", 5890, "Proxy magic for image generation APIs.",
  "ImageProxy extends the proxy to /v1/images/generations. Cost tracking per image, prompt-hash caching, and provider failover for DALL-E, Stable Diffusion, etc.",
  ["Image generation API proxy", "Per-image cost tracking", "Prompt hash caching", "Provider failover", "Size and quality controls", "Dashboard with generation history"],
  `port: 5890
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
imageproxy:
  cache: true
  default_size: 1024x1024
  max_per_day: 100
  cost_per_image:
    dall-e-3: 0.04
    dall-e-2: 0.02`],

["langbridge", "LangBridge", 5900, "Multilingual LLM middleware.",
  "LangBridge detects input language, translates to English for the model, and translates the response back. Cached translations reduce cost.",
  ["Automatic language detection", "Input translation to English", "Response translation back to user language", "Translation caching", "Language pair cost tracking", "Configurable source/target languages"],
  `port: 5900
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
langbridge:
  enabled: true
  model_language: en
  cache_translations: true
  supported_languages: [es, fr, de, ja, ko, zh, pt, it]`],

["contextwindow", "ContextWindow", 5910, "Visual context window debugger.",
  "ContextWindow provides a dashboard that visualizes token allocation across system prompt, history, context, and response budget.",
  ["Token allocation visualization", "Per-section breakdown", "Bar chart and treemap views", "Truncation point highlighting", "Optimization recommendations", "Works with any model"],
  `port: 5910
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
contextwindow:
  enabled: true
  track_sections: true`],

["regionroute", "RegionRoute", 5920, "Data residency routing for GDPR.",
  "RegionRoute routes requests to region-specific provider endpoints based on tenant, header, or IP geolocation. Keep EU data in EU.",
  ["Region-based request routing", "Header/tenant/IP geolocation routing", "Map regions to endpoints", "ComplianceLog integration", "GDPR data residency compliance", "Dashboard with regional traffic"],
  `port: 5920
providers:
  openai_us:
    base_url: https://api.openai.com/v1
    api_key: \${OPENAI_API_KEY}
  openai_eu:
    base_url: https://eu.api.openai.com/v1
    api_key: \${OPENAI_API_KEY_EU}
regionroute:
  rules:
    - region: EU
      provider: openai_eu
    - region: "*"
      provider: openai_us
  detect_by: header  # header | ip | tenant`],

// ─── PHASE 3 P3 ──────────────────────────────────────────────
["chainforge", "ChainForge", 5930, "Multi-step LLM workflows as YAML.",
  "ChainForge defines multi-step LLM pipelines in YAML. Chain extract, analyze, summarize, and format steps with conditional branching.",
  ["YAML pipeline definitions", "Data passing between steps", "Conditional branching", "Parallel execution", "Per-pipeline cost tracking", "Dashboard with pipeline monitor"],
  `port: 5930
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
pipelines:
  summarize_and_translate:
    steps:
      - name: summarize
        model: gpt-4o
        prompt: "Summarize: {{input}}"
      - name: translate
        model: gpt-4o-mini
        prompt: "Translate to Spanish: {{summarize.output}}"`],

["cronllm", "CronLLM", 5940, "Scheduled LLM tasks.",
  "CronLLM runs LLM prompts on cron schedules. Daily summaries, weekly reports, periodic content generation — all through the proxy chain.",
  ["Cron-scheduled LLM calls", "YAML job definitions", "Output to file, webhook, or email", "Full proxy chain per job", "Job history and logs", "Dashboard with schedule overview"],
  `port: 5940
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
cron:
  jobs:
    - name: daily_summary
      schedule: "0 9 * * *"
      model: gpt-4o-mini
      prompt: "Summarize yesterday's key events in tech."
      output: webhook
      webhook_url: \${SLACK_WEBHOOK}`],

["webhookrelay", "WebhookRelay", 5950, "Trigger LLM calls from any webhook.",
  "WebhookRelay exposes inbound webhook endpoints that extract data, build prompts, call LLMs, and send results to destinations.",
  ["Inbound webhook endpoints", "Configurable data extraction", "Template-based prompt building", "Result forwarding to webhooks", "GitHub/Slack/Discord triggers", "Dashboard with trigger history"],
  `port: 5950
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
webhooks:
  github_summarize:
    trigger: /webhook/github
    extract: "body.issue.body"
    prompt: "Summarize this GitHub issue: {{extracted}}"
    model: gpt-4o-mini
    forward: \${SLACK_WEBHOOK}`],

["billsync", "BillSync", 5960, "Per-customer LLM invoicing.",
  "BillSync generates per-customer invoices from UsagePulse data. Configure markup, billing periods, and export to Stripe or CSV.",
  ["Per-customer invoice generation", "Configurable markup percentages", "Billing period management", "Stripe usage record export", "CSV/PDF invoice export", "Dashboard with billing overview"],
  `port: 5960
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
billing:
  markup: 1.5           # 50% markup
  period: monthly
  currency: usd
  stripe_key: \${STRIPE_KEY}
  invoice_format: pdf`],

["whitelabel", "WhiteLabel", 5970, "Your brand on Stockyard's engine.",
  "WhiteLabel replaces Stockyard branding with yours. Custom logos, colors, domain, and product name in the dashboard.",
  ["Custom logo upload", "Brand color configuration", "Product name override", "Custom domain support", "CSS customization", "Suite top-tier only"],
  `port: 5970
whitelabel:
  brand_name: "YourBrand AI"
  logo_url: "/assets/your-logo.svg"
  primary_color: "#3B82F6"
  accent_color: "#10B981"
  custom_domain: "ai.yourbrand.com"`],

["trainexport", "TrainExport", 5980, "Export conversations as fine-tuning datasets.",
  "TrainExport filters and exports logged LLM conversations in training data formats: OpenAI JSONL, Anthropic, ShareGPT, and Alpaca.",
  ["Export as OpenAI JSONL", "Anthropic format support", "ShareGPT and Alpaca formats", "Quality filters", "PII redaction on export", "CLI and API modes"],
  `port: 5980
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
export:
  format: openai_jsonl  # openai_jsonl | anthropic | sharegpt | alpaca
  min_quality_score: 0.7
  redact_pii: true
  output_dir: ./training_data/`],

["synthgen", "SynthGen", 5990, "Generate synthetic training data.",
  "SynthGen generates synthetic training data through the proxy with quality control. Templates, seed examples, deduplication, and EvalGate scoring.",
  ["Template-based generation", "Seed example expansion", "Quality scoring per sample", "Deduplication", "Batch generation via BatchQueue", "Export in training formats"],
  `port: 5990
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
synthgen:
  template: "Generate a customer support conversation about {{topic}}"
  topics: [billing, shipping, returns, product_info]
  samples_per_topic: 100
  min_quality: 0.8
  deduplicate: true`],

["diffprompt", "DiffPrompt", 6000, "Git-style diff for prompt changes.",
  "DiffPrompt compares two prompt versions against the same test inputs with side-by-side output diffs and quality scoring.",
  ["Side-by-side prompt comparison", "Shared test input sets", "Output diff visualization", "Quality scoring per version", "Cost comparison", "CLI and dashboard modes"],
  `port: 6000
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
diffprompt:
  test_inputs:
    - "What is machine learning?"
    - "Explain recursion."
    - "Write a haiku about coding."`],

["llmbench", "LLMBench", 6010, "Benchmark any model on YOUR workload.",
  "LLMBench runs your test suite across N models and produces comparison reports on quality, latency, cost, and tokens.",
  ["Multi-model benchmarking", "Custom test suites", "Quality/latency/cost comparison", "Automated report generation", "Statistical analysis", "CLI and dashboard modes"],
  `port: 6010
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
  anthropic:
    api_key: \${ANTHROPIC_API_KEY}
bench:
  models:
    - gpt-4o
    - gpt-4o-mini
    - claude-sonnet-4-20250514
  test_suite: ./benchmarks/
  runs_per_test: 3`],

["maskmode", "MaskMode", 6020, "Demo mode with realistic fake data.",
  "MaskMode replaces real PII with realistic fakes for sales demos. Consistent mapping within sessions — same input always gets the same fake.",
  ["Realistic fake data substitution", "Consistent mapping per session", "Names, emails, phones, addresses", "Demo-safe output", "Session-scoped consistency", "Toggle on/off per request"],
  `port: 6020
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
maskmode:
  enabled: true
  locale: en_US
  fields: [name, email, phone, address, ssn]`],

["tokenmarket", "TokenMarket", 6030, "Reallocate unused API capacity across teams.",
  "TokenMarket allows teams to request additional token budget from underutilized pools. Auto-rebalance with priority queuing.",
  ["Budget pools per team", "Capacity request workflow", "Auto-rebalance unused budget", "Priority queuing", "Usage forecasting", "Dashboard with pool status"],
  `port: 6030
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
tokenmarket:
  pools:
    engineering: { budget: 100000, priority: high }
    marketing: { budget: 50000, priority: medium }
    support: { budget: 25000, priority: low }
  rebalance_interval: 1h`],

["llmsync", "LLMSync", 6040, "Sync configs across environments.",
  "LLMSync manages Stockyard configuration across dev, staging, and production with inheritance, diffs, and promotion.",
  ["Environment hierarchy", "Config inheritance with overrides", "Cross-environment diff", "Promote and rollback", "Git-friendly format", "CLI tool"],
  `port: 6040
environments:
  dev:
    extends: base
    overrides:
      costcap.daily: 1.00
  staging:
    extends: base
    overrides:
      costcap.daily: 10.00
  production:
    extends: base
    overrides:
      costcap.daily: 100.00`],

["clustermode", "ClusterMode", 6050, "Multi-instance Stockyard with shared state.",
  "ClusterMode runs multiple Stockyard instances with shared state via LiteFS replication or gossip protocol.",
  ["Multi-instance coordination", "LiteFS SQLite replication", "Leader-follower architecture", "Shared cache across instances", "Health-based leader election", "Dashboard with cluster status"],
  `port: 6050
cluster:
  enabled: true
  mode: litefs  # litefs | gossip
  peers:
    - stockyard-1:6050
    - stockyard-2:6050
  leader_election: true`],

["encryptvault", "EncryptVault", 6060, "End-to-end encryption for LLM payloads.",
  "EncryptVault encrypts sensitive fields in SQLite storage at rest. Customer-managed keys (BYOK) for healthcare, legal, and financial compliance.",
  ["Field-level encryption at rest", "Customer-managed keys (BYOK)", "AES-256-GCM encryption", "Key rotation support", "Encrypted audit logs", "HIPAA/SOC2 compliance helper"],
  `port: 6060
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
encryption:
  enabled: true
  algorithm: aes-256-gcm
  key: \${ENCRYPTION_KEY}
  encrypt_fields: [request_body, response_body]
  key_rotation_days: 90`],

["mirrortest", "MirrorTest", 6070, "Shadow test new models against production.",
  "MirrorTest sends production traffic to a shadow model for comparison. Primary response goes to the user; shadow is logged for analysis.",
  ["Shadow model testing", "Zero user impact", "Quality comparison logging", "Latency and cost comparison", "Configurable shadow percentage", "Dashboard with comparison results"],
  `port: 6070
providers:
  openai:
    api_key: \${OPENAI_API_KEY}
mirror:
  primary: gpt-4o
  shadow: gpt-4o-mini
  shadow_percent: 10  # test 10% of traffic
  log_comparison: true`],

// ─── PHASE 4 ──────────────────────────────────────────────────
...phase4Products(),
];

function phase4Products() {
  const p4 = [
["extractml", "ExtractML", 6080, "Force structured data from free-text responses.",
  "ExtractML auto-injects extraction calls when models return prose instead of structured data.",
  ["Auto-extract structured data from prose", "Define output schemas", "Retry with extraction prompt", "Cache extraction patterns", "Works with any model", "Dashboard with extraction stats"],
  `port: 6080\nextractml:\n  schema:\n    type: object\n    properties:\n      name: { type: string }\n      age: { type: integer }`],

["tableforge", "TableForge", 6090, "LLM-powered table generation with validation.",
  "TableForge validates tabular/CSV output from LLMs. Checks columns, data types, completeness, and auto-repairs.",
  ["Table output detection", "Column validation", "Data type checking", "Completeness scoring", "Auto-repair malformed rows", "CSV/JSON export"],
  `port: 6090\ntableforge:\n  expected_columns: [name, email, role]\n  types: { name: string, email: email, role: string }\n  require_complete: true`],

["toolrouter", "ToolRouter", 6100, "Manage and version LLM function calls.",
  "ToolRouter provides a registry for LLM tools with versioning, routing, shadow testing, and usage analytics.",
  ["Tool schema registry", "Version management", "Shadow testing for tool changes", "Per-tool usage analytics", "Route calls by model capability", "Dashboard with tool metrics"],
  `port: 6100\ntoolrouter:\n  registry:\n    get_weather:\n      version: v2\n      schema: { type: object, properties: { location: { type: string } } }`],

["toolshield", "ToolShield", 6110, "Validate tool calls before execution.",
  "ToolShield intercepts LLM tool_use calls and validates arguments, enforces permissions, and rate limits per tool.",
  ["Tool call argument validation", "Per-tool permissions", "Per-tool rate limits", "Block dangerous patterns", "Audit trail for tool calls", "Dashboard with tool call log"],
  `port: 6110\ntoolshield:\n  rules:\n    delete_user: { blocked: true }\n    send_email: { rate_limit: 10/hour }\n    read_file: { allowed_paths: ["/safe/"] }`],

["toolmock", "ToolMock", 6120, "Fake tool responses for testing.",
  "ToolMock intercepts tool_result messages and returns canned responses. Test tool-use agents without real external services.",
  ["Canned tool responses", "Match by tool name and arguments", "Simulate errors and timeouts", "Partial result simulation", "Deterministic for CI", "Zero external dependencies"],
  `port: 6120\ntoolmock:\n  mocks:\n    get_weather:\n      response: { temp: 72, condition: sunny }\n    search:\n      response: { results: [{ title: "Mock result" }] }`],

["authgate", "AuthGate", 6130, "API key management for YOUR users.",
  "AuthGate lets you issue, revoke, and manage API keys for your customers. Per-key usage limits and scoping.",
  ["Issue and revoke customer API keys", "Per-key usage limits", "Key scoping by model/endpoint", "Key usage dashboard", "REST API for key management", "Self-service key portal"],
  `port: 6130\nauthgate:\n  enabled: true\n  keys:\n    - id: customer-1\n      key: sk-cust-abc123\n      limits: { daily_tokens: 100000 }`],

["scopeguard", "ScopeGuard", 6140, "Fine-grained permissions per API key.",
  "ScopeGuard enforces role-based permissions on API keys. Control which models, endpoints, and features each key can access.",
  ["Role-based access control", "Model access restrictions", "Endpoint permissions", "Feature gating per key", "Token budget per role", "Audit log for denials"],
  `port: 6140\nscopeguard:\n  roles:\n    free: { models: [gpt-4o-mini], max_tokens: 1000 }\n    pro: { models: [gpt-4o, gpt-4o-mini], max_tokens: 10000 }\n    admin: { models: ["*"], max_tokens: 0 }`],

["visionproxy", "VisionProxy", 6150, "Proxy for vision/image-understanding APIs.",
  "VisionProxy handles GPT-4V and Claude vision requests with image-specific caching, cost tracking, resizing, and failover.",
  ["Image input detection and hashing", "Vision-specific caching", "Per-image cost tracking", "Auto-resize and compress", "Provider failover for vision", "Dashboard with vision metrics"],
  `port: 6150\nvisionproxy:\n  cache: true\n  resize_max: 2048\n  compress_quality: 85\n  cost_per_image: { gpt-4o: 0.01 }`],

["audioproxy", "AudioProxy", 6160, "Proxy for speech-to-text and TTS APIs.",
  "AudioProxy provides caching, cost tracking, and failover for Whisper STT and ElevenLabs/OpenAI TTS calls.",
  ["TTS response caching (text hash to audio)", "Per-minute STT cost tracking", "Provider failover for audio", "Format conversion", "Latency optimization", "Dashboard with audio metrics"],
  `port: 6160\naudioproxy:\n  tts_cache: true\n  stt_providers:\n    - whisper\n  tts_providers:\n    - openai\n    - elevenlabs`],

["docparse", "DocParse", 6170, "Preprocess documents for LLM context.",
  "DocParse extracts text from PDFs, Word docs, and HTML. Smart chunking and artifact cleaning before documents hit the LLM.",
  ["PDF text extraction", "Word doc parsing", "HTML cleaning", "Smart chunking strategies", "Artifact removal", "REST API for uploads"],
  `port: 6170\ndocparse:\n  chunk_size: 1000\n  chunk_overlap: 200\n  clean_artifacts: true\n  supported: [pdf, docx, html, txt, md]`],

["framegrab", "FrameGrab", 6180, "Video frame extraction for vision LLMs.",
  "FrameGrab extracts frames from video, batches them through vision LLMs, and caches analyses. Smart frame selection by scene change.",
  ["Scene-based frame extraction", "Batch frame analysis", "Per-frame caching", "Smart frame selection", "Cost per frame tracking", "Pipeline API"],
  `port: 6180\nframegrab:\n  extract_mode: scene_change\n  max_frames: 20\n  cache: true\n  vision_model: gpt-4o`],

["sessionstore", "SessionStore", 6190, "Managed conversation sessions.",
  "SessionStore provides CRUD operations for conversation sessions. Create, resume, list, delete, share, and export sessions.",
  ["Session CRUD API", "Full history persistence", "Metadata per session", "Concurrent session limits", "Session sharing", "Export as training data"],
  `port: 6190\nsessionstore:\n  max_sessions_per_user: 100\n  max_history: 1000\n  auto_expire: 30d`],

["convofork", "ConvoFork", 6200, "Branch conversations. Try different paths.",
  "ConvoFork lets users branch conversations at any message. Each fork has independent history. Tree visualization in dashboard.",
  ["Fork at any message", "Independent history per branch", "Tree visualization", "Compare branch outcomes", "Merge branches", "API for fork management"],
  `port: 6200\nconvofork:\n  max_forks_per_session: 10\n  max_depth: 5`],

["slotfill", "SlotFill", 6210, "Form-filling conversation engine.",
  "SlotFill provides declarative form-through-conversation. Define slots with types and validation, track fill state, auto-reprompt.",
  ["Declarative slot definitions", "Type validation per slot", "Auto-reprompt for missing slots", "Completion funnel tracking", "Custom validation functions", "Dashboard with fill rates"],
  `port: 6210\nslotfill:\n  forms:\n    booking:\n      slots:\n        - { name: date, type: date, required: true }\n        - { name: guests, type: integer, min: 1, max: 20 }\n        - { name: notes, type: string, required: false }`],

["semanticcache", "SemanticCache", 6220, "Cache hits for similar prompts.",
  "SemanticCache embeds prompts and uses cosine similarity to match. 'Weather in NYC' and 'weather in New York City' become cache hits.",
  ["Embedding-based prompt similarity", "Cosine similarity matching", "Configurable similarity threshold", "10x cache hit rate vs exact match", "EmbedCache integration", "Dashboard with similarity scores"],
  `port: 6220\nsemanticcache:\n  threshold: 0.92\n  embed_model: text-embedding-3-small\n  max_entries: 50000`],

["partialcache", "PartialCache", 6230, "Cache reusable prompt prefixes.",
  "PartialCache detects static prompt prefixes and uses native prefix caching where supported. Simulates for providers that don't support it.",
  ["Detect static prompt prefixes", "Native prefix caching support", "Simulation for unsupported providers", "Per-prefix savings tracking", "Auto-detect cacheable prefixes", "Dashboard with prefix cache stats"],
  `port: 6230\npartialcache:\n  enabled: true\n  min_prefix_tokens: 100\n  auto_detect: true`],

["streamcache", "StreamCache", 6240, "Cache streaming responses with timing.",
  "StreamCache stores original SSE chunk timing. Cache hits replay with realistic timing so chat UIs look natural.",
  ["Store original chunk timing", "Realistic timing replay", "Instant mode option", "Streaming-aware cache", "Natural UX on cache hits", "Dashboard with replay stats"],
  `port: 6240\nstreamcache:\n  store_timing: true\n  replay_mode: realistic  # realistic | instant\n  ttl: 3600`],

["promptchain", "PromptChain", 6250, "Composable prompt blocks.",
  "PromptChain manages reusable prompt components. Compose system prompts from blocks: [tone.helpful, format.json, domain.ecommerce].",
  ["Reusable prompt components", "Compose blocks into prompts", "Auto-update across products", "Version per block", "Dependency tracking", "Dashboard with block usage"],
  `port: 6250\npromptchain:\n  blocks:\n    tone.helpful: "You are a helpful, concise assistant."\n    format.json: "Always respond in valid JSON."\n    domain.support: "You handle customer support for our SaaS."\n  compose:\n    default: [tone.helpful, format.json]`],

["promptfuzz", "PromptFuzz", 6260, "Fuzz-test your prompts.",
  "PromptFuzz generates adversarial, multilingual, and edge case inputs to stress-test prompts. Score with EvalGate.",
  ["Adversarial input generation", "Multilingual test cases", "Edge case discovery", "EvalGate scoring integration", "Failure report generation", "CLI and API modes"],
  `port: 6260\npromptfuzz:\n  categories: [adversarial, multilingual, edge_case, injection]\n  runs_per_category: 50\n  target_prompt: "You are a helpful assistant."`],

["promptmarket", "PromptMarket", 6270, "Community prompt library.",
  "PromptMarket provides a public prompt library. Publish, browse, rate, and fork community prompts.",
  ["Publish and browse prompts", "Rating system", "Fork and customize", "Usage tracking", "Category organization", "Free adoption driver"],
  `port: 6270\npromptmarket:\n  enabled: true\n  allow_publish: true\n  require_rating: true`],

["costpredict", "CostPredict", 6280, "Predict request cost before sending.",
  "CostPredict counts input tokens and estimates output to calculate cost before the request is sent. Adds X-Estimated-Cost header.",
  ["Pre-send cost estimation", "Input token counting", "Output estimation", "X-Estimated-Cost header", "Optional block on high cost", "Dashboard with prediction accuracy"],
  `port: 6280\ncostpredict:\n  enabled: true\n  block_above: 1.00\n  output_estimate_ratio: 0.5`],

["costmap", "CostMap", 6290, "Multi-dimensional cost attribution.",
  "CostMap tags requests with dimensions and provides interactive drill-down cost analytics.",
  ["Tag-based cost attribution", "Multi-dimensional drill-down", "Interactive dashboard", "Export to BI tools", "Per-feature cost tracking", "Budget vs actual per dimension"],
  `port: 6290\ncostmap:\n  dimensions:\n    - header: x-feature\n    - header: x-team\n    - model`],

["spotprice", "SpotPrice", 6300, "Real-time model pricing intelligence.",
  "SpotPrice maintains a live pricing database and routes to the cheapest model meeting quality thresholds.",
  ["Live pricing database", "Cost-optimized model selection", "Quality threshold enforcement", "Price alert notifications", "Historical price tracking", "Dashboard with price trends"],
  `port: 6300\nspotprice:\n  min_quality_score: 0.8\n  update_interval: 1h\n  prefer_cheapest: true`],

["loadforge", "LoadForge", 6310, "Load test your LLM stack.",
  "LoadForge runs LLM-specific load tests measuring TTFT, tokens per second, streaming stability, and error rates under load.",
  ["LLM-specific load profiles", "TTFT measurement", "Tokens per second tracking", "Streaming stability testing", "p50/p95/p99 reporting", "CLI with HTML report"],
  `port: 6310\nloadforge:\n  profile:\n    concurrent: 50\n    duration: 60s\n    ramp_up: 10s\n  target: http://localhost:4000/v1`],

["snapshottest", "SnapshotTest", 6320, "Snapshot testing for LLM outputs.",
  "SnapshotTest records baseline LLM responses and compares future outputs with semantic diffing and configurable thresholds.",
  ["Record baseline responses", "Semantic diff (not exact)", "Configurable similarity threshold", "CI-friendly exit codes", "Update snapshots command", "Regression detection"],
  `port: 6320\nsnapshottest:\n  baseline_dir: ./snapshots/\n  threshold: 0.85\n  update_command: "snapshottest --update"`],

["chaosllm", "ChaosLLM", 6330, "Chaos engineering for LLM stacks.",
  "ChaosLLM injects realistic failures: 429s, timeouts, malformed JSON, truncated streams. Test your error handling.",
  ["Inject 429 rate limit errors", "Simulate timeouts", "Return malformed JSON", "Truncate streaming responses", "Configurable failure rates", "Dashboard with chaos stats"],
  `port: 6330\nchaosllm:\n  failure_rate: 0.1  # 10% of requests fail\n  failures:\n    - type: 429\n      weight: 40\n    - type: timeout\n      weight: 30\n    - type: malformed_json\n      weight: 20\n    - type: truncated_stream\n      weight: 10`],

["datamap", "DataMap", 6340, "GDPR Article 30 data flow mapping.",
  "DataMap auto-classifies data flowing through the proxy and generates GDPR-required records of processing activities.",
  ["Auto-classify personal data", "Map data flows per provider", "GDPR Article 30 records", "Processing activity export", "Data category tagging", "Dashboard with flow visualization"],
  `port: 6340\ndatamap:\n  enabled: true\n  classify_pii: true\n  export_format: json`],

["consentgate", "ConsentGate", 6350, "User consent management for AI.",
  "ConsentGate checks per-user consent status before allowing AI processing. Blocks non-consented requests. Supports withdrawal.",
  ["Per-user consent tracking", "Block non-consented requests", "Consent timestamp recording", "Withdrawal support", "EU AI Act compliance", "Dashboard with consent status"],
  `port: 6350\nconsentgate:\n  enabled: true\n  consent_header: X-User-Consent\n  block_without_consent: true`],

["retentionwipe", "RetentionWipe", 6360, "Automated data retention and deletion.",
  "RetentionWipe enforces data retention periods and handles GDPR right-to-erasure requests with deletion certificates.",
  ["Configurable retention periods", "Auto-purge expired data", "Per-user deletion API", "Deletion certificates", "GDPR right-to-erasure", "Dashboard with retention status"],
  `port: 6360\nretentionwipe:\n  retention:\n    logs: 90d\n    analytics: 365d\n    audit: 730d\n  deletion_api: true`],

["policyengine", "PolicyEngine", 6370, "Codify AI governance as enforceable rules.",
  "PolicyEngine compiles YAML governance policies into middleware rules. Audit compliance rates across all products.",
  ["YAML policy definitions", "Compile to middleware rules", "Compliance rate tracking", "Policy violation audit log", "Cross-product enforcement", "Dashboard with compliance scores"],
  `port: 6370\npolicies:\n  - name: no_pii_to_cloud\n    rule: "if provider.type == cloud then require promptguard.enabled"\n  - name: log_everything\n    rule: "require compliancelog.enabled"`],

["streamsplit", "StreamSplit", 6380, "Fork streams to multiple destinations.",
  "StreamSplit tees live SSE streams to multiple consumers: user, logger, quality checker, webhook. Zero latency for primary.",
  ["Tee SSE to multiple destinations", "Zero latency for primary consumer", "Configurable destinations", "Per-destination filtering", "Webhook forwarding", "Dashboard with split stats"],
  `port: 6380\nstreamsplit:\n  destinations:\n    - type: primary\n    - type: webhook\n      url: \${LOG_WEBHOOK}\n    - type: quality_check`],

["streamthrottle", "StreamThrottle", 6390, "Control streaming speed.",
  "StreamThrottle limits tokens per second in streaming responses. Buffer fast streams for better UX or reading speed.",
  ["Max tokens per second", "Buffer fast streams", "Configurable per endpoint", "Model-specific speeds", "Client-specific throttling", "Dashboard with speed metrics"],
  `port: 6390\nstreamthrottle:\n  max_tokens_per_sec: 30\n  buffer: true`],

["streamtransform", "StreamTransform", 6400, "Transform streams mid-flight.",
  "StreamTransform applies transformation pipelines to streaming chunks: strip markdown, redact PII, translate in real-time.",
  ["Mid-stream transformations", "Strip markdown", "Real-time PII redaction", "Translation pipeline", "Minimal latency impact", "Configurable pipeline"],
  `port: 6400\nstreamtransform:\n  pipeline:\n    - strip_markdown\n    - redact_pii\n  buffer_size: 5  # chunks`],

["modelalias", "ModelAlias", 6410, "Abstract away model names.",
  "ModelAlias maps friendly names to specific model versions. Change the underlying model without updating 50 configs.",
  ["Friendly model name aliases", "Central model mapping", "Change without config updates", "Version pinning", "Alias history", "Dashboard with alias usage"],
  `port: 6410\naliases:\n  fast: gpt-4o-mini\n  smart: gpt-4o\n  cheap: gpt-3.5-turbo\n  best: claude-sonnet-4-20250514`],

["paramnorm", "ParamNorm", 6420, "Normalize parameters across providers.",
  "ParamNorm calibrates temperature, top_p, and other parameters across models so the same settings produce similar behavior.",
  ["Cross-model parameter calibration", "Temperature normalization", "Top_p mapping", "Per-model calibration profiles", "Consistent behavior across providers", "Dashboard with parameter mapping"],
  `port: 6420\nparamnorm:\n  calibration:\n    temperature:\n      gpt-4o: { scale: 1.0 }\n      claude-sonnet: { scale: 0.8 }\n      gemini-pro: { scale: 1.2 }`],

["quotasync", "QuotaSync", 6430, "Track provider rate limits in real-time.",
  "QuotaSync parses rate limit headers from provider responses and tracks remaining quota per model and endpoint.",
  ["Parse rate limit headers", "Track remaining quota", "Per-model quota tracking", "Near-limit alerts", "Provider-specific parsing", "Dashboard with quota status"],
  `port: 6430\nquotasync:\n  track_providers: [openai, anthropic]\n  alert_at_percent: 80`],

["errornorm", "ErrorNorm", 6440, "Normalize errors across providers.",
  "ErrorNorm translates all provider error responses into a single consistent schema with error codes, retry hints, and provider context.",
  ["Unified error schema", "Error code normalization", "Retry-after extraction", "Is-retryable flag", "Provider context preservation", "Dashboard with error analytics"],
  `port: 6440\nerrornorm:\n  enabled: true\n  schema: { error_code: int, message: string, provider: string, retry_after: int, retryable: bool }`],

["cohorttrack", "CohortTrack", 6450, "User cohort analytics for LLM products.",
  "CohortTrack groups users into cohorts by signup date, plan, or feature and tracks retention, cost, and engagement per cohort.",
  ["Cohort grouping (signup, plan, feature)", "Retention tracking", "Cost per cohort", "Engagement metrics", "BI export", "Dashboard with cohort charts"],
  `port: 6450\ncohorttrack:\n  dimensions: [signup_month, plan, feature]\n  retention_windows: [7d, 30d, 90d]`],

["promptrank", "PromptRank", 6460, "Rank prompts by ROI.",
  "PromptRank combines cost, quality score, latency, volume, and feedback into a per-template ROI index. Find your best and worst prompts.",
  ["Per-template ROI index", "Cost/quality/latency ranking", "Volume-weighted scoring", "Feedback integration", "Prompt leaderboard", "Dashboard with ROI charts"],
  `port: 6460\npromptrank:\n  metrics: [cost, quality, latency, volume, feedback]\n  weight: { quality: 0.4, cost: 0.3, latency: 0.2, volume: 0.1 }`],

["anomalyradar", "AnomalyRadar", 6470, "ML-powered anomaly detection.",
  "AnomalyRadar builds statistical baselines for latency, cost, and errors. Z-score deviation detection with auto-adjusting thresholds.",
  ["Statistical baseline building", "Z-score deviation detection", "Auto-adjusting thresholds", "Multi-metric monitoring", "Alert on anomalies", "Dashboard with anomaly timeline"],
  `port: 6470\nanomalyradar:\n  metrics: [latency, cost, error_rate, token_volume]\n  sensitivity: 2.5  # z-score threshold\n  baseline_window: 7d`],

["envsync", "EnvSync", 6480, "Sync configs and secrets across environments.",
  "EnvSync manages full environment configs including encrypted secrets with push, promote, diff, and rollback.",
  ["Config + secrets management", "Encrypted secret storage", "Push/promote/diff/rollback", "Pre-promotion validation", "Environment hierarchy", "CLI tool"],
  `port: 6480\nenvsync:\n  environments: [dev, staging, production]\n  secret_encryption: true\n  promotion_chain: dev -> staging -> production`],

["proxylog", "ProxyLog", 6490, "Structured logging for every proxy decision.",
  "ProxyLog instruments each middleware to emit decision logs. See WHY provider B was chosen over A, WHY a cache miss happened.",
  ["Per-middleware decision logging", "X-Proxy-Trace header", "Full request decision trace", "Searchable decision history", "Middleware timing breakdown", "Dashboard with decision explorer"],
  `port: 6490\nproxylog:\n  enabled: true\n  log_decisions: true\n  trace_header: X-Proxy-Trace\n  retention_days: 30`],

["clidash", "CliDash", 6500, "Terminal dashboard for your LLM stack.",
  "CliDash provides an htop-style terminal UI for monitoring Stockyard. Real-time req/sec, cache stats, spend, and errors.",
  ["Terminal-native monitoring (TUI)", "Real-time metrics display", "Keyboard drill-down", "SSH-accessible", "No browser needed", "bubbletea-based rendering"],
  `port: 6500\nclidash:\n  refresh_interval: 1s\n  panels: [requests, cache, spend, errors, models]`],

["embedrouter", "EmbedRouter", 6510, "Smart routing for embedding requests.",
  "EmbedRouter collects embedding requests over a time window, deduplicates, batches, and routes by content type.",
  ["Time-window request collection", "Automatic deduplication", "Batch optimization", "Content-type routing", "Per-caller response mapping", "Dashboard with batch stats"],
  `port: 6510\nembedrouter:\n  window_ms: 50\n  deduplicate: true\n  batch_size: 100`],

["finetunetrack", "FineTuneTrack", 6520, "Monitor fine-tuned model performance.",
  "FineTuneTrack runs evaluation suites against your fine-tuned models periodically. Track scores and alert on degradation.",
  ["Periodic evaluation runs", "Score tracking over time", "Base model comparison", "Degradation alerts", "Data distribution monitoring", "Dashboard with performance trends"],
  `port: 6520\nfinetunetrack:\n  models:\n    - id: ft:gpt-4o-mini:my-org:custom:id\n      eval_suite: ./evals/\n      schedule: "0 0 * * 0"  # weekly\n      baseline: gpt-4o-mini`],

["agentreplay", "AgentReplay", 6530, "Record and replay agent sessions.",
  "AgentReplay reconstructs full agent sessions from TraceLink data. Step-by-step playback with what-if mode.",
  ["Session reconstruction from traces", "Step-by-step playback", "What-if mode (change a step, replay)", "Export as test cases", "Decision tree visualization", "Dashboard with session explorer"],
  `port: 6530\nagentreplay:\n  source: tracelink\n  max_session_length: 100\n  export_format: jsonl`],

["summarizegate", "SummarizeGate", 6540, "Auto-summarize long contexts to save tokens.",
  "SummarizeGate scores relevance per context section. Keeps high-relevance verbatim, summarizes low-relevance to save tokens.",
  ["Per-section relevance scoring", "Selective summarization", "Token savings tracking", "Configurable relevance threshold", "Preserve high-value content", "Dashboard with savings stats"],
  `port: 6540\nsummarizegate:\n  relevance_threshold: 0.6\n  summarize_model: gpt-4o-mini\n  max_summary_ratio: 0.3`],

["codelang", "CodeLang", 6550, "Language-aware code validation.",
  "CodeLang uses tree-sitter parsing for actual syntax validation of LLM-generated code. Finds undefined references and suspicious patterns.",
  ["Tree-sitter based parsing", "Syntax error detection", "Undefined reference checking", "Language-specific rules", "Multiple language support", "Dashboard with code quality metrics"],
  `port: 6550\ncodelang:\n  languages: [python, javascript, go, rust]\n  checks: [syntax, undefined_refs, suspicious_patterns]`],

["personaswitch", "PersonaSwitch", 6560, "Hot-swap AI personalities.",
  "PersonaSwitch manages named personality profiles with prompt, temperature, and format rules. Route by header, key, or user segment.",
  ["Named persona profiles", "Per-persona system prompts", "Temperature and format per persona", "Route by header or key", "A/B testing personas", "Dashboard with persona usage"],
  `port: 6560\npersonas:\n  formal:\n    system_prompt: "You are a professional business assistant."\n    temperature: 0.3\n  casual:\n    system_prompt: "You're a friendly, casual helper. Keep it chill."\n    temperature: 0.8\ndefault: formal\nroute_by: header  # X-Persona header`],

["warmpool", "WarmPool", 6570, "Pre-warm model connections.",
  "WarmPool maintains persistent connections to providers and keeps local models loaded. Eliminates cold start latency.",
  ["Persistent provider connections", "Keep-alive for local models", "Health check maintenance", "Connection pooling", "Cold start elimination", "Dashboard with connection status"],
  `port: 6570\nwarmpool:\n  providers:\n    - openai\n    - ollama\n  health_interval: 30s\n  keep_alive: true`],

["edgecache", "EdgeCache", 6580, "CDN-like distributed caching.",
  "EdgeCache distributes cached LLM responses across multiple instances via LiteFS or optional Redis. Geographic cache hit optimization.",
  ["Cross-instance cache sharing", "LiteFS replication", "Optional Redis backend", "Geographic hit rate tracking", "Cache invalidation across nodes", "Dashboard with distribution stats"],
  `port: 6580\nedgecache:\n  backend: litefs  # litefs | redis\n  redis_url: ""\n  replication_lag_max: 100ms`],

["queuepriority", "QueuePriority", 6590, "Priority queues for LLM requests.",
  "QueuePriority extends BatchQueue with priority levels. Enterprise customers jump ahead of free tier. Reserved capacity and SLA tracking.",
  ["Priority levels per key/tenant", "Reserved capacity", "SLA tracking", "Queue depth per priority", "Auto-promote on timeout", "Dashboard with queue analytics"],
  `port: 6590\nqueuepriority:\n  levels:\n    critical: { weight: 10, reserved: 5 }\n    high: { weight: 5 }\n    normal: { weight: 1 }\n    low: { weight: 0 }`],

["geoprice", "GeoPrice", 6600, "Purchasing power pricing by region.",
  "GeoPrice adjusts pricing based on user region using purchasing power parity. Anti-VPN detection included.",
  ["PPP-adjusted pricing", "Region detection", "Anti-VPN checks", "Revenue by region tracking", "Configurable multipliers", "Dashboard with regional revenue"],
  `port: 6600\ngeoprice:\n  multipliers:\n    US: 1.0\n    EU: 0.9\n    BR: 0.4\n    IN: 0.3\n  detect_vpn: true`],

["tokenauction", "TokenAuction", 6610, "Dynamic pricing based on demand.",
  "TokenAuction adjusts per-request pricing based on queue depth, time of day, and provider costs. Surge pricing for peak demand.",
  ["Demand-based pricing", "Time-of-day adjustments", "Surge pricing rules", "Provider cost tracking", "Revenue optimization", "Dashboard with pricing trends"],
  `port: 6610\ntokenauction:\n  base_price: 0.001\n  surge_multiplier: 2.0\n  surge_threshold: 0.8  # queue 80% full`],

["canarydeploy", "CanaryDeploy", 6620, "Canary deployments for model changes.",
  "CanaryDeploy gradually rolls out new models: 5% to 25% to 100%. Auto-promote on quality, auto-rollback on degradation.",
  ["Gradual traffic shifting", "Auto-promote on quality", "Auto-rollback on degradation", "Configurable stages", "Quality comparison", "Dashboard with rollout status"],
  `port: 6620\ncanary:\n  old: gpt-4o\n  new: gpt-4o-2025-02\n  stages: [5, 25, 50, 100]\n  promote_threshold: 0.95\n  rollback_threshold: 0.80`],

["playbackstudio", "PlaybackStudio", 6630, "Interactive playground for logged requests.",
  "PlaybackStudio provides rich exploration of logged interactions. Advanced filters, conversation threads, side-by-side comparison, and content search.",
  ["Advanced request filtering", "Conversation thread view", "Side-by-side comparison", "Content search", "Bulk actions", "Interactive dashboard"],
  `port: 6630\nplaybackstudio:\n  source: promptreplay\n  search_index: true\n  max_results: 1000`],

["webhookforge", "WebhookForge", 6640, "Visual webhook-to-LLM pipeline builder.",
  "WebhookForge provides a visual flow builder for multi-step webhook-triggered LLM pipelines with conditional branching.",
  ["Visual flow builder", "Multi-step pipelines", "Conditional branching", "Execution history", "Template library", "Dashboard with flow editor"],
  `port: 6640\nwebhookforge:\n  enabled: true\n  max_pipelines: 50\n  execution_timeout: 60s`],

["mirrortest2", "MirrorTest", 6070, "Shadow test new models against production.",
  "MirrorTest sends production traffic to a shadow model. Primary response returned to user; shadow logged for analysis.",
  ["Shadow model testing", "Zero user impact", "Quality comparison", "Latency/cost comparison", "Configurable shadow percentage", "Dashboard with comparison results"],
  `port: 6070\nmirror:\n  primary: gpt-4o\n  shadow: gpt-4o-mini\n  shadow_percent: 10`],
  ];
  // Remove the duplicate mirrortest2
  return p4.filter(p => p[0] !== "mirrortest2");
}

// ─── GENERATION ─────────────────────────────────────────────

for (const product of PRODUCTS) {
  const [binary, name, port, tagline, desc, features, configYaml] = product;

  // README
  const readmePath = path.join(CMD_DIR, binary, "README.md");
  if (fs.existsSync(readmePath)) {
    skipR++;
  } else {
    if (!fs.existsSync(path.join(CMD_DIR, binary))) {
      fs.mkdirSync(path.join(CMD_DIR, binary), { recursive: true });
    }
    const featureList = features.map(f => `- ${f}`).join("\n");
    const readmeContent = `# ${name}

**${tagline}**

${desc}

## Quickstart

\`\`\`bash
export OPENAI_API_KEY=sk-...
npx @stockyard/${binary}

# Your app:   http://localhost:${port}/v1/chat/completions
# Dashboard:  http://localhost:${port}/ui
\`\`\`

## What You Get

${featureList}

## Config

\`\`\`yaml
# ${binary}.yaml
${configYaml}
\`\`\`

## Docker

\`\`\`bash
docker run -p ${port}:${port} -e OPENAI_API_KEY=sk-... stockyard/${binary}
\`\`\`

## Part of Stockyard

${name} is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ${name} standalone.
`;
    fs.writeFileSync(readmePath, readmeContent);
    readmes++;
  }

  // Config
  const configName = binary === "stockyard" ? "stockyard.example.yaml" : `${binary}.example.yaml`;
  const configPath = path.join(CFG_DIR, configName);
  if (fs.existsSync(configPath)) {
    skipC++;
  } else {
    const configContent = `# ${name} — ${tagline}
# Copy to ${binary}.yaml and set your API keys.

${configYaml}

# Common settings
data_dir: ~/.stockyard
log_level: info
`;
    fs.writeFileSync(configPath, configContent);
    configs++;
  }
}

console.log(`\nREADMEs:  ${readmes} created, ${skipR} already existed`);
console.log(`Configs:  ${configs} created, ${skipC} already existed`);
console.log(`Total:    ${readmes + skipR} READMEs, ${configs + skipC} configs`);
