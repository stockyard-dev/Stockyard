#!/usr/bin/env node
/**
 * Generate MCP server packages for all Stockyard products that don't have them yet.
 * Run: node generate-all-mcp.js
 */

const fs = require("fs");
const path = require("path");

// All 93 products that need MCP servers (Phase 3 P2 + P3 + Phase 4)
// Format: [key, displayName, port, tagline, description, icon, keywords, tools[]]
const NEW_PRODUCTS = [
  // ── Phase 3 P2 (21) ──
  ["agentguard", "AgentGuard", 5690, "Safety rails for autonomous AI agents",
    "Per-session limits for AI agents: max calls, cost, duration. Kill runaway agent sessions before they drain your budget.",
    "🛡️", ["agent", "safety", "session", "limits", "autonomous", "cost-control"],
    [
      { name: "agentguard_sessions", desc: "List active agent sessions with call counts, cost, and duration." },
      { name: "agentguard_kill", desc: "Kill a specific agent session by ID." },
      { name: "agentguard_stats", desc: "Get aggregate stats: sessions tracked, killed, costs saved." },
    ]],
  ["codefence", "CodeFence", 5700, "Validate LLM-generated code before it runs",
    "Scan LLM code output for dangerous patterns: shell injection, file access, crypto mining. Block or flag unsafe code.",
    "🔒", ["code", "security", "validation", "sandbox", "patterns"],
    [
      { name: "codefence_stats", desc: "Get code validation stats: scanned, flagged, blocked." },
      { name: "codefence_patterns", desc: "List active forbidden patterns." },
      { name: "codefence_add_pattern", desc: "Add a custom forbidden code pattern.", method: "POST" },
    ]],
  ["hallucicheck", "HalluciCheck", 5710, "Catch LLM hallucinations before your users do",
    "Validate URLs, emails, and citations in LLM responses. Flag or retry when models invent non-existent references.",
    "🔍", ["hallucination", "validation", "urls", "fact-check", "quality"],
    [
      { name: "hallucicheck_stats", desc: "Get hallucination detection stats: checked, invalid URLs/emails found." },
      { name: "hallucicheck_recent", desc: "List recent hallucination detections with details." },
    ]],
  ["tierdrop", "TierDrop", 5720, "Auto-downgrade models when burning cash",
    "Gracefully degrade from GPT-4 to GPT-3.5 when approaching budget limits. Cost-aware model selection.",
    "📉", ["cost", "downgrade", "model", "budget", "tier"],
    [
      { name: "tierdrop_stats", desc: "Get downgrade stats: triggers, models switched, savings." },
      { name: "tierdrop_tiers", desc: "List configured cost tiers and thresholds." },
    ]],
  ["driftwatch", "DriftWatch", 5730, "Detect when model behavior changes",
    "Track latency and output patterns per model over time. Alert when behavior drifts beyond thresholds.",
    "📊", ["drift", "monitoring", "quality", "baseline", "regression"],
    [
      { name: "driftwatch_stats", desc: "Get drift detection stats per model." },
      { name: "driftwatch_baselines", desc: "View current baselines for tracked models." },
    ]],
  ["feedbackloop", "FeedbackLoop", 5740, "Close the LLM improvement loop",
    "Collect user ratings and feedback linked to specific LLM requests. Track quality trends over time.",
    "👍", ["feedback", "ratings", "quality", "improvement"],
    [
      { name: "feedbackloop_stats", desc: "Get feedback stats: total ratings, average score, trends." },
      { name: "feedbackloop_submit", desc: "Submit feedback for a request.", method: "POST" },
      { name: "feedbackloop_recent", desc: "List recent feedback entries." },
    ]],
  ["abrouter", "ABRouter", 5750, "A/B test any LLM variable with statistical rigor",
    "Run experiments across models, prompts, temperatures. Weighted traffic splits with automatic significance testing.",
    "🧪", ["ab-test", "experiment", "statistics", "split", "optimization"],
    [
      { name: "abrouter_experiments", desc: "List active experiments with variant stats." },
      { name: "abrouter_create", desc: "Create a new A/B experiment.", method: "POST" },
      { name: "abrouter_results", desc: "Get statistical results for an experiment." },
    ]],
  ["guardrail", "GuardRail", 5760, "Keep your LLM on-script",
    "Topic fencing middleware. Define allowed/denied topics. Block off-topic responses with custom fallback messages.",
    "🚧", ["guardrail", "topic", "filter", "boundary", "safety"],
    [
      { name: "guardrail_stats", desc: "Get topic enforcement stats: blocked, allowed, violations." },
      { name: "guardrail_topics", desc: "List allowed and denied topic patterns." },
    ]],
  ["geminishim", "GeminiShim", 5770, "Tame Gemini's quirks behind clean API",
    "Handle Gemini safety filter blocks with auto-retry. Normalize token counts. OpenAI-compatible surface for Gemini.",
    "♊", ["gemini", "google", "compatibility", "shim", "safety-filter"],
    [
      { name: "geminishim_stats", desc: "Get Gemini compatibility stats: retries, safety blocks, normalizations." },
    ]],
  ["localsync", "LocalSync", 5780, "Seamlessly blend local and cloud models",
    "Route to Ollama locally when available. Auto-failover to cloud when local is down. Track cost savings.",
    "🏠", ["local", "ollama", "hybrid", "failover", "cost"],
    [
      { name: "localsync_stats", desc: "Get routing stats: local vs cloud, savings, failovers." },
      { name: "localsync_health", desc: "Check local endpoint health." },
    ]],
  ["devproxy", "DevProxy", 5790, "Charles Proxy for LLM APIs",
    "Interactive debugging proxy. Log headers, bodies, latency for every request. Development inspection tool.",
    "🔧", ["debug", "inspect", "development", "logging", "headers"],
    [
      { name: "devproxy_stats", desc: "Get debug stats: requests logged, avg latency." },
      { name: "devproxy_recent", desc: "List recent requests with headers and timing." },
    ]],
  ["promptslim", "PromptSlim", 5800, "Compress prompts by 40-70% without losing meaning",
    "Remove redundant whitespace, filler words, articles. Configurable aggressiveness. See before/after token savings.",
    "✂️", ["prompt", "compression", "tokens", "cost", "optimization"],
    [
      { name: "promptslim_stats", desc: "Get compression stats: chars saved, tokens saved, compression ratio." },
    ]],
  ["promptlint", "PromptLint", 5810, "Catch prompt anti-patterns before they cost you money",
    "Static analysis for prompts: detect redundancy, injection patterns, excessive length. Score and suggest improvements.",
    "🔎", ["prompt", "lint", "analysis", "quality", "patterns"],
    [
      { name: "promptlint_stats", desc: "Get lint stats: issues found by severity, top patterns." },
    ]],
  ["approvalgate", "ApprovalGate", 5820, "Require human approval for prompt changes",
    "Approval workflow for prompt modifications. Track who approved what and when. Audit trail included.",
    "✅", ["approval", "workflow", "governance", "audit"],
    [
      { name: "approvalgate_stats", desc: "Get approval stats: pending, approved, rejected." },
      { name: "approvalgate_pending", desc: "List pending approval requests." },
    ]],
  ["outputcap", "OutputCap", 5830, "Stop paying for responses you don't need",
    "Cap output length at natural sentence boundaries. No more 500-token essays when you asked for one word.",
    "📏", ["output", "length", "cap", "cost", "truncation"],
    [
      { name: "outputcap_stats", desc: "Get capping stats: tokens saved, avg reduction." },
    ]],
  ["agegate", "AgeGate", 5840, "Child safety middleware for LLM apps",
    "Age-appropriate content filtering. Tiers: child, teen, adult. Injects safety prompts, filters output. COPPA/KOSA ready.",
    "👶", ["child-safety", "age", "coppa", "filter", "content"],
    [
      { name: "agegate_stats", desc: "Get safety stats: content filtered, tier distribution." },
    ]],
  ["voicebridge", "VoiceBridge", 5850, "LLM middleware for voice/TTS pipelines",
    "Strip markdown, URLs, code blocks from responses. Convert to speakable prose for voice assistants.",
    "🎙️", ["voice", "tts", "speech", "markdown", "cleanup"],
    [
      { name: "voicebridge_stats", desc: "Get voice optimization stats: elements stripped, avg length." },
    ]],
  ["imageproxy", "ImageProxy", 5860, "Proxy magic for image generation APIs",
    "Cost tracking, caching, and failover for DALL-E and other image generation APIs.",
    "🎨", ["image", "dalle", "generation", "cache", "cost"],
    [
      { name: "imageproxy_stats", desc: "Get image proxy stats: requests, cache hits, cost." },
    ]],
  ["langbridge", "LangBridge", 5870, "Cross-language translation for multilingual apps",
    "Auto-detect language, translate to English for model, translate response back. Seamless multilingual support.",
    "🌐", ["translation", "multilingual", "language", "i18n"],
    [
      { name: "langbridge_stats", desc: "Get translation stats: languages detected, translations performed." },
    ]],
  ["contextwindow", "ContextWindow", 5880, "Visual context window debugger",
    "Visualize token allocation by message role. See what's eating your context window. Optimization recommendations.",
    "🪟", ["context", "tokens", "debug", "visualization", "optimization"],
    [
      { name: "contextwindow_stats", desc: "Get context window analysis: breakdown by role, total usage." },
    ]],
  ["regionroute", "RegionRoute", 5890, "Data residency routing for GDPR compliance",
    "Route requests to region-specific endpoints. Keep EU data in EU. Geographic compliance made easy.",
    "🌍", ["gdpr", "region", "routing", "compliance", "data-residency"],
    [
      { name: "regionroute_stats", desc: "Get routing stats: requests per region." },
      { name: "regionroute_routes", desc: "List configured region routes." },
    ]],

  // ── Phase 3 P3 (15) ──
  ["chainforge", "ChainForge", 5900, "Multi-step LLM workflows as YAML pipelines",
    "Define extract→analyze→summarize→format pipelines. Conditional branching, parallel execution, cost tracking per pipeline.",
    "⛓️", ["pipeline", "workflow", "chain", "multi-step", "orchestration"],
    [
      { name: "chainforge_stats", desc: "Get pipeline execution stats." },
      { name: "chainforge_pipelines", desc: "List configured pipelines." },
    ]],
  ["cronllm", "CronLLM", 5910, "Scheduled LLM tasks — your AI cron job runner",
    "Define scheduled prompts in YAML. Daily summaries, weekly reports, periodic checks. Runs through full proxy chain.",
    "⏰", ["cron", "schedule", "automation", "tasks", "periodic"],
    [
      { name: "cronllm_stats", desc: "Get job execution stats." },
      { name: "cronllm_jobs", desc: "List scheduled jobs." },
    ]],
  ["webhookrelay", "WebhookRelay", 5920, "Trigger LLM calls from any webhook",
    "Receive webhooks, extract data, build prompts, call LLM, send results. GitHub→summarize→Slack in one config.",
    "🔗", ["webhook", "trigger", "event-driven", "automation"],
    [
      { name: "webhookrelay_stats", desc: "Get relay stats: webhooks received, calls triggered." },
      { name: "webhookrelay_triggers", desc: "List configured webhook triggers." },
    ]],
  ["billsync", "BillSync", 5930, "Per-customer LLM invoices automatically",
    "Track usage per tenant. Apply markup. Generate invoice data. Stripe-compatible usage records.",
    "💰", ["billing", "invoice", "tenant", "markup", "saas"],
    [
      { name: "billsync_stats", desc: "Get billing stats: tenants, revenue, markup." },
      { name: "billsync_tenants", desc: "List tenant billing summaries." },
    ]],
  ["whitelabel", "WhiteLabel", 5940, "Your brand on Stockyard's engine",
    "Custom branding for resellers. Logo, colors, domain. Sell LLM infrastructure under your own brand.",
    "🏷️", ["branding", "white-label", "reseller", "custom"],
    [
      { name: "whitelabel_stats", desc: "Get branding stats." },
    ]],
  ["trainexport", "TrainExport", 5950, "Export LLM conversations as fine-tuning datasets",
    "Collect input/output pairs from live traffic. Export as OpenAI JSONL, Anthropic, or Alpaca format.",
    "📤", ["training", "fine-tuning", "export", "dataset", "jsonl"],
    [
      { name: "trainexport_stats", desc: "Get collection stats: pairs collected, storage used." },
      { name: "trainexport_export", desc: "Export collected pairs in specified format.", method: "POST" },
    ]],
  ["synthgen", "SynthGen", 5960, "Generate synthetic training data through your proxy",
    "Templates + seed examples → synthetic training data at scale. Quality-checked through EvalGate.",
    "🧬", ["synthetic", "training", "generation", "data"],
    [
      { name: "synthgen_stats", desc: "Get generation stats: samples generated, batches run." },
    ]],
  ["diffprompt", "DiffPrompt", 5970, "Git-style diff for prompt changes",
    "Track system prompt changes. Hash-based detection. See which models had prompt modifications.",
    "📝", ["diff", "prompt", "versioning", "change-detection"],
    [
      { name: "diffprompt_stats", desc: "Get change detection stats: prompts checked, changes detected." },
    ]],
  ["llmbench", "LLMBench", 5980, "Benchmark any model on YOUR workload",
    "Per-model performance tracking: latency, cost, tokens. Compare models on your actual traffic.",
    "🏋️", ["benchmark", "performance", "comparison", "latency", "cost"],
    [
      { name: "llmbench_stats", desc: "Get benchmark results per model." },
      { name: "llmbench_compare", desc: "Compare two models side by side." },
    ]],
  ["maskmode", "MaskMode", 5990, "Demo mode with realistic fake data",
    "Replace real PII in responses with realistic fakes. Consistent within session. Perfect for sales demos.",
    "🎭", ["demo", "mask", "pii", "fake-data", "sales"],
    [
      { name: "maskmode_stats", desc: "Get masking stats: requests masked, replacements made." },
    ]],
  ["tokenmarket", "TokenMarket", 6000, "Dynamic budget reallocation across teams",
    "Pool-based budgets. Teams request capacity. Auto-rebalance. Priority queuing for high-value requests.",
    "🏪", ["budget", "pool", "reallocation", "teams"],
    [
      { name: "tokenmarket_stats", desc: "Get market stats: pool balances, transactions." },
      { name: "tokenmarket_pools", desc: "List budget pools with current balances." },
    ]],
  ["llmsync", "LLMSync", 6010, "Replicate config across environments",
    "Environment hierarchy with config inheritance. Diff, promote, rollback. Git-friendly YAML management.",
    "🔄", ["sync", "config", "environment", "deployment"],
    [
      { name: "llmsync_stats", desc: "Get sync stats." },
    ]],
  ["clustermode", "ClusterMode", 6020, "Run multiple instances with shared state",
    "Multi-instance coordination. Leader-follower with shared cache. Scale beyond single-instance SQLite.",
    "🏗️", ["cluster", "scale", "multi-instance", "coordination"],
    [
      { name: "clustermode_stats", desc: "Get cluster stats: nodes, requests distributed." },
      { name: "clustermode_nodes", desc: "List cluster nodes and their status." },
    ]],
  ["encryptvault", "EncryptVault", 6030, "End-to-end encryption for sensitive LLM payloads",
    "AES-GCM encryption for sensitive fields. Customer-managed keys. HIPAA/SOC2 compliance ready.",
    "🔐", ["encryption", "security", "hipaa", "compliance", "vault"],
    [
      { name: "encryptvault_stats", desc: "Get encryption stats: fields encrypted/decrypted." },
    ]],
  ["mirrortest", "MirrorTest", 6040, "Shadow test new models against production traffic",
    "Send production traffic to a shadow model. Compare quality, latency, cost. Zero user impact.",
    "🪞", ["shadow", "testing", "comparison", "canary"],
    [
      { name: "mirrortest_stats", desc: "Get shadow test stats: requests mirrored, success rates." },
    ]],

  // ── Phase 4 (57) ──
  ["extractml", "ExtractML", 6050, "Turn unstructured LLM responses into structured data", "Force extraction from free-text into JSON when models return prose.", "🧲", ["extraction", "structured", "json", "parsing"],
    [{ name: "extractml_stats", desc: "Get extraction stats." }]],
  ["tableforge", "TableForge", 6060, "LLM-powered CSV/table generation with validation", "Detect tables in output. Validate columns, types, completeness. Auto-repair and export.", "📊", ["table", "csv", "validation", "structured"],
    [{ name: "tableforge_stats", desc: "Get table validation stats." }]],
  ["toolrouter", "ToolRouter", 6070, "Manage, version, and route LLM function calls", "Versioned tool schemas. Route calls. Shadow-test. Usage analytics.", "🔀", ["tools", "function-calling", "routing", "versioning"],
    [{ name: "toolrouter_stats", desc: "Get tool routing stats." }, { name: "toolrouter_tools", desc: "List registered tools." }]],
  ["toolshield", "ToolShield", 6080, "Validate and sandbox LLM tool calls", "Intercept tool_use. Validate args. Per-tool permissions and rate limits.", "🛡️", ["tools", "validation", "sandbox", "permissions"],
    [{ name: "toolshield_stats", desc: "Get tool validation stats." }]],
  ["toolmock", "ToolMock", 6090, "Fake tool responses for testing", "Canned responses by tool+args. Simulate errors, timeouts, partial results.", "🃏", ["tools", "mock", "testing", "simulation"],
    [{ name: "toolmock_stats", desc: "Get mock stats." }]],
  ["authgate", "AuthGate", 6100, "API key management for YOUR users", "Issue/revoke keys to your customers. Per-key limits and usage tracking.", "🔑", ["auth", "api-keys", "management", "customers"],
    [{ name: "authgate_stats", desc: "Get auth stats." }, { name: "authgate_keys", desc: "List API keys." }]],
  ["scopeguard", "ScopeGuard", 6110, "Fine-grained permissions per API key", "Role-based access control. Map keys to allowed models, endpoints, features.", "🎯", ["permissions", "rbac", "scope", "access-control"],
    [{ name: "scopeguard_stats", desc: "Get permission stats." }, { name: "scopeguard_roles", desc: "List roles." }]],
  ["visionproxy", "VisionProxy", 6120, "Proxy magic for vision/image APIs", "Caching, cost tracking, and failover for GPT-4V, Claude vision.", "👁️", ["vision", "image", "multimodal", "cache"],
    [{ name: "visionproxy_stats", desc: "Get vision proxy stats." }]],
  ["audioproxy", "AudioProxy", 6130, "Proxy for speech-to-text and text-to-speech", "Cache TTS, track per-minute costs, failover between STT/TTS providers.", "🔊", ["audio", "stt", "tts", "speech", "whisper"],
    [{ name: "audioproxy_stats", desc: "Get audio proxy stats." }]],
  ["docparse", "DocParse", 6140, "Preprocess documents before they hit the LLM", "PDF/Word/HTML text extraction. Smart chunking. Clean artifacts.", "📄", ["document", "parsing", "chunking", "pdf", "rag"],
    [{ name: "docparse_stats", desc: "Get document processing stats." }]],
  ["framegrab", "FrameGrab", 6150, "Extract and analyze video frames through vision LLMs", "Scene detection. Batch frames. Smart frame selection. Cost per frame.", "🎬", ["video", "frames", "vision", "analysis"],
    [{ name: "framegrab_stats", desc: "Get frame extraction stats." }]],
  ["sessionstore", "SessionStore", 6160, "Managed conversation sessions", "Create/resume/list/delete sessions. Full history. Metadata. Concurrent limits.", "💬", ["session", "conversation", "history", "management"],
    [{ name: "sessionstore_stats", desc: "Get session stats." }, { name: "sessionstore_sessions", desc: "List active sessions." }]],
  ["convofork", "ConvoFork", 6170, "Branch conversations — try different paths", "Fork at any message. Independent history per branch. Tree visualization.", "🌿", ["fork", "branch", "conversation", "parallel"],
    [{ name: "convofork_stats", desc: "Get fork stats." }]],
  ["slotfill", "SlotFill", 6180, "Form-filling conversation engine", "Declarative slot definitions. Track filled/missing. Reprompt. Completion funnels.", "📋", ["forms", "slots", "conversation", "intake"],
    [{ name: "slotfill_stats", desc: "Get slot fill stats." }]],
  ["semanticcache", "SemanticCache", 6190, "Cache hits for similar prompts, not just identical", "Embed prompts. Cosine similarity. Configurable threshold. 10x hit rate.", "🧠", ["cache", "semantic", "similarity", "embeddings"],
    [{ name: "semanticcache_stats", desc: "Get semantic cache stats." }]],
  ["partialcache", "PartialCache", 6200, "Cache reusable prompt prefixes", "Detect static system prompt prefix. Use native prefix caching where supported.", "🧩", ["cache", "prefix", "optimization", "tokens"],
    [{ name: "partialcache_stats", desc: "Get prefix cache stats." }]],
  ["streamcache", "StreamCache", 6210, "Cache streaming responses with realistic timing", "Store original chunk timing. Replay cached SSE with original pacing.", "📺", ["cache", "streaming", "sse", "replay"],
    [{ name: "streamcache_stats", desc: "Get stream cache stats." }]],
  ["promptchain", "PromptChain", 6220, "Composable prompt blocks", "Define reusable blocks. Compose: [tone.helpful, format.json, domain.ecommerce]. Auto-update.", "🔗", ["prompt", "components", "composable", "reusable"],
    [{ name: "promptchain_stats", desc: "Get composition stats." }, { name: "promptchain_blocks", desc: "List defined blocks." }]],
  ["promptfuzz", "PromptFuzz", 6230, "Fuzz-test your prompts", "Generate adversarial, multilingual, edge-case inputs. Score with EvalGate. Report failures.", "🐛", ["fuzz", "testing", "adversarial", "prompt", "security"],
    [{ name: "promptfuzz_stats", desc: "Get fuzz test stats." }]],
  ["promptmarket", "PromptMarket", 6240, "Community prompt library", "Publish, browse, rate, fork prompts. Track which community prompts you use.", "🏪", ["prompt", "marketplace", "community", "sharing"],
    [{ name: "promptmarket_stats", desc: "Get marketplace stats." }]],
  ["costpredict", "CostPredict", 6250, "Predict request cost BEFORE sending", "Count input tokens. Estimate output. Calculate cost. X-Estimated-Cost header.", "🔮", ["cost", "prediction", "estimate", "budget"],
    [{ name: "costpredict_stats", desc: "Get prediction stats." }]],
  ["costmap", "CostMap", 6260, "Multi-dimensional cost attribution", "Tag requests with dimensions. Drill-down: by feature, user, prompt.", "🗺️", ["cost", "attribution", "analytics", "drill-down"],
    [{ name: "costmap_stats", desc: "Get cost attribution stats." }]],
  ["spotprice", "SpotPrice", 6270, "Real-time model pricing intelligence", "Live pricing DB. Route to cheapest model meeting quality threshold.", "💱", ["pricing", "cost", "routing", "optimization"],
    [{ name: "spotprice_stats", desc: "Get pricing stats." }]],
  ["loadforge", "LoadForge", 6280, "Load test your LLM stack", "Define load profiles. Measure TTFT, TPS, p50/p95/p99, errors.", "⚡", ["load-test", "performance", "benchmark", "stress"],
    [{ name: "loadforge_stats", desc: "Get load test results." }]],
  ["snapshottest", "SnapshotTest", 6290, "Snapshot testing for LLM outputs", "Record baselines. Semantic diff. Configurable threshold. CI-friendly.", "📸", ["snapshot", "testing", "regression", "ci-cd"],
    [{ name: "snapshottest_stats", desc: "Get snapshot test stats." }]],
  ["chaosllm", "ChaosLLM", 6300, "Chaos engineering for your LLM stack", "Inject realistic failures: 429s, timeouts, malformed JSON, truncated streams.", "💥", ["chaos", "testing", "resilience", "fault-injection"],
    [{ name: "chaosllm_stats", desc: "Get chaos injection stats." }]],
  ["datamap", "DataMap", 6310, "GDPR Article 30 data flow mapping", "Auto-classify data. Map flows: source→proxy→provider→storage. Generate GDPR records.", "🗃️", ["gdpr", "compliance", "data-flow", "mapping"],
    [{ name: "datamap_stats", desc: "Get data mapping stats." }]],
  ["consentgate", "ConsentGate", 6320, "User consent management for AI interactions", "Check consent per user. Block non-consented. Track timestamps. Support withdrawal.", "✋", ["consent", "gdpr", "eu-ai-act", "compliance"],
    [{ name: "consentgate_stats", desc: "Get consent stats." }]],
  ["retentionwipe", "RetentionWipe", 6330, "Automated data retention and deletion", "Retention periods per data type. Auto-purge. Per-user deletion. Deletion certificates.", "🧹", ["retention", "deletion", "gdpr", "compliance"],
    [{ name: "retentionwipe_stats", desc: "Get retention stats." }]],
  ["policyengine", "PolicyEngine", 6340, "Codify AI governance as enforceable rules", "YAML policy rules compiled to middleware. Audit log. Compliance rate dashboard.", "📜", ["policy", "governance", "compliance", "rules"],
    [{ name: "policyengine_stats", desc: "Get policy enforcement stats." }]],
  ["streamsplit", "StreamSplit", 6350, "Fork streaming responses to multiple destinations", "Tee SSE chunks to logger, quality checker, webhook. Zero latency for primary.", "🔱", ["streaming", "fork", "multiplex", "sse"],
    [{ name: "streamsplit_stats", desc: "Get stream split stats." }]],
  ["streamthrottle", "StreamThrottle", 6360, "Control streaming speed for better UX", "Max tokens/sec. Buffer fast streams. Per endpoint/model/client.", "🚦", ["streaming", "throttle", "speed", "ux"],
    [{ name: "streamthrottle_stats", desc: "Get throttle stats." }]],
  ["streamtransform", "StreamTransform", 6370, "Transform streaming responses mid-stream", "Pipeline on chunks: strip markdown, redact PII, translate. Minimal latency.", "🔄", ["streaming", "transform", "pipeline", "real-time"],
    [{ name: "streamtransform_stats", desc: "Get transform stats." }]],
  ["modelalias", "ModelAlias", 6380, "Abstract away model names with aliases", "Aliases: fast→gpt-4o-mini, smart→claude-sonnet. Change mapping, all apps update.", "🏷️", ["alias", "model", "abstraction", "mapping"],
    [{ name: "modelalias_stats", desc: "Get alias resolution stats." }, { name: "modelalias_list", desc: "List active aliases." }]],
  ["paramnorm", "ParamNorm", 6390, "Normalize parameters across providers", "Calibration profiles per model. Map normalized params to model-specific ranges.", "⚖️", ["parameters", "normalization", "calibration"],
    [{ name: "paramnorm_stats", desc: "Get normalization stats." }]],
  ["quotasync", "QuotaSync", 6400, "Track provider rate limits in real-time", "Parse rate limit headers. Track per model/endpoint. Alert near limits.", "📈", ["quota", "rate-limit", "tracking", "provider"],
    [{ name: "quotasync_stats", desc: "Get quota tracking stats." }]],
  ["errornorm", "ErrorNorm", 6410, "Normalize error responses across providers", "Single error schema: code, message, provider, retry_after, is_retryable.", "⚠️", ["errors", "normalization", "consistency"],
    [{ name: "errornorm_stats", desc: "Get error normalization stats." }]],
  ["cohorttrack", "CohortTrack", 6420, "User cohort analytics for LLM products", "Cohorts by signup, plan, feature. Retention, cost per cohort. BI export.", "👥", ["analytics", "cohort", "retention", "users"],
    [{ name: "cohorttrack_stats", desc: "Get cohort analytics." }]],
  ["promptrank", "PromptRank", 6430, "Rank prompts by ROI", "Per template: cost, quality, latency, volume, feedback. ROI leaderboard.", "🏆", ["analytics", "prompt", "roi", "ranking"],
    [{ name: "promptrank_stats", desc: "Get prompt rankings." }]],
  ["anomalyradar", "AnomalyRadar", 6440, "ML-powered anomaly detection", "Build statistical baselines. Z-score deviation detection. Auto-adjusting thresholds.", "📡", ["anomaly", "detection", "monitoring", "ml"],
    [{ name: "anomalyradar_stats", desc: "Get anomaly detection stats." }]],
  ["envsync", "EnvSync", 6450, "Sync configs + secrets across environments", "Push/promote/diff. Encrypted secrets. Pre-promotion validation. Rollback.", "🔐", ["sync", "secrets", "environment", "deployment"],
    [{ name: "envsync_stats", desc: "Get sync stats." }]],
  ["proxylog", "ProxyLog", 6460, "Structured logging for every proxy decision", "Each middleware emits decision log. Per-request trace. X-Proxy-Trace header.", "📋", ["logging", "decisions", "trace", "debug"],
    [{ name: "proxylog_stats", desc: "Get logging stats." }]],
  ["clidash", "CliDash", 6470, "Terminal dashboard — htop for your LLM stack", "Real-time TUI: req/sec, models, cache, spend, errors. SSH-accessible.", "🖥️", ["terminal", "dashboard", "tui", "monitoring"],
    [{ name: "clidash_stats", desc: "Get dashboard data." }]],
  ["embedrouter", "EmbedRouter", 6480, "Smart routing for embedding requests", "Batch over 50ms window. Deduplicate. Route by content type.", "🔀", ["embeddings", "routing", "batch", "dedup"],
    [{ name: "embedrouter_stats", desc: "Get embedding routing stats." }]],
  ["finetunetrack", "FineTuneTrack", 6490, "Monitor fine-tuned model performance", "Eval suite. Run periodically. Track scores. Compare to base model.", "📉", ["fine-tune", "monitoring", "evaluation", "drift"],
    [{ name: "finetunetrack_stats", desc: "Get fine-tune tracking stats." }]],
  ["agentreplay", "AgentReplay", 6500, "Record and replay agent sessions step-by-step", "Step-by-step playback on TraceLink data. What-if mode. Export as test cases.", "🎬", ["agent", "replay", "debug", "session"],
    [{ name: "agentreplay_stats", desc: "Get replay stats." }]],
  ["summarizegate", "SummarizeGate", 6510, "Auto-summarize long contexts to save tokens", "Score relevance per section. Keep high-relevance verbatim. Summarize low-relevance.", "📝", ["summarize", "context", "tokens", "optimization"],
    [{ name: "summarizegate_stats", desc: "Get summarization stats." }]],
  ["codelang", "CodeLang", 6520, "Language-aware code generation with syntax validation", "Tree-sitter parsing. Syntax errors, undefined refs, suspicious patterns.", "💻", ["code", "syntax", "validation", "parsing"],
    [{ name: "codelang_stats", desc: "Get code validation stats." }]],
  ["personaswitch", "PersonaSwitch", 6530, "Hot-swap AI personalities without code changes", "Define personas. Route by header/key/segment. Each: prompt, temperature, rules.", "🎭", ["persona", "personality", "routing", "customization"],
    [{ name: "personaswitch_stats", desc: "Get persona stats." }, { name: "personaswitch_personas", desc: "List personas." }]],
  ["warmpool", "WarmPool", 6540, "Pre-warm model connections", "Persistent connections. Health checks. Keep-alive for Ollama.", "🔥", ["warmup", "connections", "latency", "performance"],
    [{ name: "warmpool_stats", desc: "Get connection pool stats." }]],
  ["edgecache", "EdgeCache", 6550, "CDN-like caching for LLM responses", "Distribute cache across instances. Geographic hit rates.", "🌐", ["cache", "cdn", "edge", "distributed"],
    [{ name: "edgecache_stats", desc: "Get edge cache stats." }]],
  ["queuepriority", "QueuePriority", 6560, "Priority queues — VIP users first", "Priority levels per key/tenant. Reserved capacity. SLA tracking.", "👑", ["queue", "priority", "vip", "sla"],
    [{ name: "queuepriority_stats", desc: "Get queue stats." }]],
  ["geoprice", "GeoPrice", 6570, "Purchasing power pricing by region", "PPP-adjusted pricing. Anti-VPN. Revenue by region dashboard.", "💱", ["pricing", "geo", "ppp", "regional"],
    [{ name: "geoprice_stats", desc: "Get pricing stats." }]],
  ["tokenauction", "TokenAuction", 6580, "Dynamic pricing based on demand", "Monitor costs, queue, errors. Time-of-day pricing. Surge pricing.", "🏷️", ["pricing", "dynamic", "auction", "demand"],
    [{ name: "tokenauction_stats", desc: "Get auction stats." }]],
  ["canarydeploy", "CanaryDeploy", 6590, "Canary deployments for prompt/model changes", "Gradual rollout: 5%→25%→100%. Auto-promote if quality holds. Auto-rollback.", "🐤", ["canary", "deployment", "rollout", "gradual"],
    [{ name: "canarydeploy_stats", desc: "Get canary deployment stats." }]],
  ["playbackstudio", "PlaybackStudio", 6600, "Interactive playground for exploring logged interactions", "Advanced filters. Conversation threads. Side-by-side. Bulk actions.", "🎪", ["playground", "explore", "logs", "interactive"],
    [{ name: "playbackstudio_stats", desc: "Get exploration stats." }]],
  ["webhookforge", "WebhookForge", 6610, "Visual builder for webhook→LLM→action pipelines", "Visual flow builder. Trigger→transform→LLM→condition→action. History.", "⚒️", ["webhook", "builder", "visual", "pipeline"],
    [{ name: "webhookforge_stats", desc: "Get pipeline stats." }]],
];

// ─── Generate everything ─────────────────────────────────────────────
const packagesDir = path.join(__dirname, "mcp", "packages");
let productsJS = "";
let count = 0;

for (const [key, displayName, port, tagline, desc, icon, keywords, tools] of NEW_PRODUCTS) {
  const pkgDir = path.join(packagesDir, `mcp-${key}`);
  if (fs.existsSync(pkgDir)) {
    // console.log(`  skip ${key} (exists)`);
    continue;
  }

  fs.mkdirSync(pkgDir, { recursive: true });

  // index.js
  fs.writeFileSync(path.join(pkgDir, "index.js"),
`#!/usr/bin/env node
/**
 * @stockyard/mcp-${key} — ${tagline}
 * 
 * MCP server for Stockyard ${displayName}.
 * ${desc}
 * 
 * Usage: npx @stockyard/mcp-${key}
 */

const { createMCPServer } = require("../../shared/factory");

const server = createMCPServer("${key}");
server.start();
`);

  // package.json
  const allKeywords = ["mcp", "mcp-server", "llm", ...keywords, "proxy", "stockyard", "model-context-protocol", "cursor", "claude-desktop"];
  fs.writeFileSync(path.join(pkgDir, "package.json"), JSON.stringify({
    name: `@stockyard/mcp-${key}`,
    version: "0.1.0",
    description: desc,
    main: "index.js",
    bin: { [`mcp-stockyard-${key}`]: "index.js" },
    keywords: allKeywords,
    author: "Stockyard",
    license: "MIT",
    repository: { type: "git", url: `https://github.com/stockyard-dev/mcp-${key}` },
    homepage: `https://stockyard.dev/mcp/${key}`,
    engines: { node: ">=18" },
    os: ["darwin", "linux", "win32"],
    cpu: ["x64", "arm64"],
    files: ["index.js", "README.md"],
  }, null, 2) + "\n");

  // README.md
  fs.writeFileSync(path.join(pkgDir, "README.md"),
`# ${icon} @stockyard/mcp-${key}

**${displayName}** — ${tagline}

${desc}

## Quick Start

\`\`\`bash
npx @stockyard/mcp-${key}
\`\`\`

## Add to Claude Desktop / Cursor

\`\`\`json
{
  "mcpServers": {
    "stockyard-${key}": {
      "command": "npx",
      "args": ["@stockyard/mcp-${key}"],
      "env": {
        "OPENAI_API_KEY": "your-key"
      }
    }
  }
}
\`\`\`

## Tools

| Tool | Description |
|------|-------------|
| \`${key}_setup\` | Download and start the ${displayName} proxy |
${tools.map(t => `| \`${t.name}\` | ${t.desc} |`).join("\n")}
| \`${key}_configure_client\` | Get client configuration instructions |

## Part of Stockyard

${displayName} is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
`);

  // smithery.yaml
  fs.writeFileSync(path.join(pkgDir, "smithery.yaml"),
`name: stockyard-${key}
display_name: "Stockyard ${displayName}"
description: "${desc}"
icon: "${icon}"
command: npx
args:
  - "@stockyard/mcp-${key}"
env:
  OPENAI_API_KEY:
    description: "OpenAI API key"
    required: true
    secret: true
tags:
${keywords.map(k => `  - ${k}`).join("\n")}
`);

  // glama.json
  fs.writeFileSync(path.join(pkgDir, "glama.json"), JSON.stringify({
    name: `stockyard-${key}`,
    display_name: `Stockyard ${displayName}`,
    description: desc,
    repository: `https://github.com/stockyard-dev/mcp-${key}`,
    command: "npx",
    args: [`@stockyard/mcp-${key}`],
    tools: tools.map(t => ({ name: t.name, description: t.desc })),
    tags: keywords,
  }, null, 2) + "\n");

  // mcp-so.json
  fs.writeFileSync(path.join(pkgDir, "mcp-so.json"), JSON.stringify({
    name: `@stockyard/mcp-${key}`,
    title: `${displayName} — ${tagline}`,
    description: desc,
    install: `npx @stockyard/mcp-${key}`,
    config: {
      command: "npx",
      args: [`@stockyard/mcp-${key}`],
      env: { OPENAI_API_KEY: "your-key" },
    },
    tools_count: tools.length + 2, // +setup +configure_client
    categories: ["llm", "proxy", "developer-tools"],
  }, null, 2) + "\n");

  // Accumulate products.js entries
  const toolDefs = tools.map(t => {
    const base = `{ name: "${t.name}", description: "${t.desc}", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats"`;
    return t.method ? `      ${base}, method: "${t.method}" }` : `      ${base} }`;
  }).join(",\n");

  productsJS += `
  ${key}: {
    binary: "${key}",
    port: ${port},
    displayName: "${displayName}",
    tagline: "${tagline}",
    description: "${desc}",
    keywords: ${JSON.stringify(allKeywords)},
    defaultConfig: {
      port: ${port}, data_dir: "~/.stockyard", log_level: "info", product: "${key}",
      providers: { openai: { api_key: "\${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
${toolDefs},
      { name: "${key}_proxy_status", description: "Check if the ${displayName} proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
`;

  count++;
}

// Write the products extension file
fs.writeFileSync(path.join(__dirname, "mcp", "shared", "products_expansion.js"),
`/**
 * Stockyard Expansion Product Definitions (Phase 3 P2/P3 + Phase 4)
 * ${count} additional products. Merged into PRODUCTS at runtime.
 */

const EXPANSION_PRODUCTS = {${productsJS}};

module.exports = { EXPANSION_PRODUCTS };
`);

console.log(`\n✅ Generated ${count} new MCP packages`);
console.log(`   Products JS: mcp/shared/products_expansion.js`);
console.log(`   Total packages: ${fs.readdirSync(packagesDir).length}`);
