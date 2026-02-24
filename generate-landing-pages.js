#!/usr/bin/env node
/**
 * Generate conversion-focused landing pages for all 125 Stockyard products.
 * Brand: dark leather/rust palette, Libre Baskerville + JetBrains Mono
 * Target: vibecoders (indie devs using AI coding tools)
 * 
 * Run: node generate-landing-pages.js
 * Output: site/products/{key}/index.html
 */

const fs = require("fs");
const path = require("path");

// ─── Product data for all 125 products ────────────────────────────────
// [key, displayName, tagline, painPoint, solution, icon, category, features[], cta]
const PRODUCTS = [
  // Original 7
  ["costcap", "CostCap", "Never get a surprise LLM bill again", "You shipped a feature with GPT-4. Monday morning: $2,847 charge. Your credit card is crying.", "Hard spending caps per project, model, and time period. Daily/monthly limits. Real-time alerts at 50%, 80%, 100%. Auto-block when budget hits.", "💸", "Cost Control", ["Per-project spend tracking", "Hard + soft caps", "Real-time alerts via webhook", "Auto-block on budget hit", "Per-model cost breakdown", "Dashboard with burn rate charts"], "Stop the bleeding"],
  ["llmcache", "CacheLayer", "Stop paying twice for the same LLM response", "Your app sends the same 'What are your business hours?' question 500 times a day. That's $15/day for identical responses.", "Exact-match caching with configurable TTL. One line of config, instant 40-90% cost reduction on repeated queries.", "📦", "Caching", ["Exact-match response caching", "Configurable TTL per model", "Cache hit/miss dashboard", "Token savings tracking", "Zero-latency cached responses", "SQLite storage, no Redis needed"], "Cache everything"],
  ["jsonguard", "StructuredShield", "LLM responses that always parse", "JSON.parse() fails. Again. The model returned markdown instead of JSON. Your app crashes at 2am.", "JSON schema validation on every response. Auto-retry with reinforced instructions when parsing fails. Up to 3 retries.", "🛡️", "Validation", ["JSON schema validation", "Auto-retry on parse failure", "Configurable max retries", "Schema registry", "Validation stats dashboard", "Works with any JSON schema"], "Fix your JSON"],
  ["routefall", "FallbackRouter", "LLM calls that never fail", "OpenAI is down. Your entire app is dead. Your users are tweeting about it.", "Automatic failover across providers. Circuit breaker pattern. Health checks. When OpenAI drops, traffic flows to Anthropic in milliseconds.", "🔀", "Reliability", ["Multi-provider failover", "Circuit breaker pattern", "Health check monitoring", "Configurable priority chains", "Zero-downtime routing", "Provider latency tracking"], "Never go down"],
  ["rateshield", "RateShield", "Rate limiting that actually works for LLMs", "Your app gets popular. 1000 users hitting GPT-4 simultaneously. Rate limits everywhere. Errors cascade.", "Token bucket rate limiting per API key, per user, per model. Smooth traffic, no bursts. Queue overflow handling.", "🚦", "Traffic Control", ["Token bucket algorithm", "Per-key rate limits", "Per-model limits", "Burst handling", "Queue management", "Rate limit dashboard"], "Tame the traffic"],
  ["promptreplay", "PromptReplay", "Every LLM request, recorded and replayable", "Bug report: 'The AI said something weird.' You have no idea what prompt produced it.", "Full request/response logging with replay. Search by model, time, content. Export for debugging. One-click replay.", "📼", "Logging", ["Full request/response capture", "One-click replay", "Search and filter", "Export to JSON/CSV", "Session grouping", "Storage-efficient compression"], "Record everything"],
  ["stockyard", "Stockyard Suite", "125 LLM tools. One binary. $59/month.", "You need caching AND rate limiting AND cost caps AND failover AND logging AND... 5 tools at $29 each = $145/mo.", "Every Stockyard product in a single binary. One config file. One dashboard. All 125 tools for less than the price of 3.", "🏗️", "Suite", ["All 125 products included", "Single binary, single config", "Unified dashboard", "63-step middleware chain", "$0.47 per tool per month", "Everything works together"], "Get everything"],

  // Phase 1
  ["keypool", "KeyPool", "Never hit API key rate limits again", "One API key. 10,000 RPM limit. 15 developers. Everyone's getting 429s.", "Pool multiple API keys. Round-robin, least-used, or random rotation. Auto-rotate on 429. Distribute load across keys.", "🔑", "Key Management", ["Multi-key pooling", "Round-robin rotation", "Least-used strategy", "Auto-rotate on 429", "Per-key usage tracking", "Key health monitoring"], "Pool your keys"],
  ["promptguard", "PromptGuard", "PII never reaches the LLM", "User pastes their SSN into your chatbot. It goes straight to OpenAI's servers. GDPR auditors are calling.", "Regex-based PII detection and redaction. Prompt injection detection. Block, redact, or redact-and-restore modes.", "🔒", "Security", ["PII detection (SSN, email, phone, CC)", "Prompt injection detection", "Redact/block/restore modes", "Configurable sensitivity", "Pattern dashboard", "Custom regex patterns"], "Protect your data"],
  ["modelswitch", "ModelSwitch", "Smart model routing for every request", "Simple questions go to GPT-4 ($$$). Complex ones go to GPT-3.5 (garbage). No intelligence in routing.", "Route by token count, prompt patterns, headers. Send simple queries to cheap models, complex ones to powerful models. A/B testing built in.", "🧠", "Routing", ["Pattern-based routing", "Token count routing", "Header-based routing", "A/B testing", "Cost tracking per route", "Tiered model chains"], "Route smarter"],
  ["evalgate", "EvalGate", "Catch bad LLM responses before users see them", "Model returns gibberish. One-word answer to a complex question. Wrong JSON structure. Users see it all.", "Quality scoring on every response. JSON validation, length checks, regex patterns, custom validators. Auto-retry bad responses.", "✅", "Quality", ["Response quality scoring", "JSON parse validation", "Min/max length checks", "Regex validators", "Auto-retry on failure", "Quality trends dashboard"], "Gate bad responses"],
  ["usagepulse", "UsagePulse", "Know exactly who's using what", "LLM bill is $3K/month. Which feature? Which user? Which team? Nobody knows.", "Per-user, per-feature, per-team token metering. Spend caps per dimension. Billing export. Webhook alerts.", "📊", "Metering", ["Multi-dimensional metering", "Per-user tracking", "Per-feature tracking", "Spend caps per dimension", "CSV/JSON billing export", "Usage webhooks"], "Meter everything"],

  // Phase 2
  ["promptpad", "PromptPad", "Version control for your prompts", "System prompt lives in a string constant. Changed it in prod. Broke everything. No rollback.", "Versioned prompt templates with A/B testing. Roll back instantly. Compare versions. Track which version performs best.", "📝", "Prompt Management", ["Prompt versioning", "A/B testing", "Instant rollback", "Version comparison", "Performance tracking", "Template variables"], "Version your prompts"],
  ["tokentrim", "TokenTrim", "Fit more into your context window", "Context window is 8K tokens. Your system prompt + history + RAG context = 12K. Responses get weird.", "Smart truncation strategies. Trim oldest messages, least relevant context, or compress. Always fit within the window.", "✂️", "Optimization", ["Smart truncation", "Multiple strategies", "Priority-based trimming", "Token counting", "Before/after comparison", "Context window visualization"], "Trim to fit"],
  ["batchqueue", "BatchQueue", "Async LLM requests that don't overwhelm", "Need to process 10,000 documents. Fire all at once. Get rate limited. Half fail. Retry chaos.", "Async request queue with configurable concurrency. Priority levels. Progress tracking. Automatic retry on failure.", "📬", "Queueing", ["Async request queue", "Configurable concurrency", "Priority levels", "Progress tracking", "Auto-retry on failure", "Batch status dashboard"], "Queue it up"],
  ["multicall", "MultiCall", "Ask multiple models, get the best answer", "Is GPT-4 better than Claude for this? Only one way to find out: call both and compare. Every time.", "Send one request to N models. Get all responses. Compare quality, cost, latency. Consensus voting for critical decisions.", "🔄", "Comparison", ["Multi-model consensus", "Side-by-side comparison", "Quality scoring per model", "Cost comparison", "Latency comparison", "Majority voting"], "Compare models"],
  ["streamsnap", "StreamSnap", "Capture and replay SSE streams", "Streaming response looks great live. Bug reported. Can't reproduce. The stream is gone.", "Capture full SSE streams with timing. Replay with original pacing. TTFT metrics. Stream debugging made easy.", "📡", "Streaming", ["Full SSE stream capture", "Replay with original timing", "TTFT metrics", "Stream debugging", "Export stream data", "Per-stream analytics"], "Capture streams"],
  ["llmtap", "LLMTap", "Full analytics for your LLM stack", "Running blind. No idea about p95 latency, error rates, cost trends. Just vibes.", "Complete analytics portal. p50/p95/p99 latency. Cost trends. Error rates. Model comparison. Real-time dashboard.", "📈", "Analytics", ["p50/p95/p99 latency", "Cost trend analysis", "Error rate tracking", "Model comparison", "Real-time dashboard", "Historical data export"], "See everything"],
  ["contextpack", "ContextPack", "Poor man's RAG in 30 seconds", "Need to give your LLM context from files, databases, URLs. Building a RAG pipeline is a 2-week project.", "Inject context from local files, SQLite queries, or URLs directly into prompts. Zero infrastructure. Config-only RAG.", "📎", "Context", ["File context injection", "SQLite query context", "URL content injection", "Keyword-based selection", "Token-aware truncation", "Zero infrastructure RAG"], "Add context instantly"],
  ["retrypilot", "RetryPilot", "Intelligent retry that doesn't make things worse", "Naive retry on 429. All retries hit at the same time. Thundering herd. Everything gets worse.", "Exponential backoff with jitter. Circuit breaker. Model downgrade on persistent failure. Smart retry that actually helps.", "🔁", "Reliability", ["Exponential backoff + jitter", "Circuit breaker pattern", "Model downgrade fallback", "Configurable retry budget", "Per-error-type strategies", "Retry analytics"], "Retry smarter"],

  // Phase 3 P1 (12)
  ["toxicfilter", "ToxicFilter", "Content moderation for LLM outputs", "Chatbot generates harmful content. Customer screenshots it. It's on Twitter.", "Rule-based output filtering. Keyword lists, regex patterns, webhook classifiers. Block, redact, or flag toxic content.", "🚫", "Safety", ["Output content filtering", "Keyword + regex rules", "Block/redact/flag modes", "Webhook classifiers", "Moderation dashboard", "Custom rule definitions"], "Filter toxicity"],
  ["compliancelog", "ComplianceLog", "Immutable audit trail for every LLM call", "SOC2 auditor asks: 'What did your AI tell customer X on March 15th?' You have no answer.", "Append-only audit log with hash chains. Tamper detection. Configurable retention. HIPAA/SOC2 compliance-ready export.", "📋", "Compliance", ["Append-only logging", "Hash chain integrity", "Tamper detection", "Retention policies", "Compliance export formats", "Per-interaction audit trail"], "Prove compliance"],
  ["secretscan", "SecretScan", "Catch API keys leaking through your LLM", "Developer pastes AWS key into prompt. LLM echoes secrets from training data. Keys leak both directions.", "Scan requests AND responses for API keys, tokens, passwords. TruffleHog-style pattern library. Block or redact instantly.", "🔐", "Security", ["Bidirectional scanning", "API key detection", "Token/password patterns", "Block or redact", "Alert on detection", "Pattern library"], "Stop key leaks"],
  ["tracelink", "TraceLink", "Distributed tracing for LLM chains", "Agent makes 12 LLM calls for one user question. Can't tell which call caused the bad answer.", "Propagate trace IDs across requests. Link parent-child calls. Waterfall visualization. OpenTelemetry compatible.", "🔗", "Observability", ["Trace ID propagation", "Parent-child linking", "Waterfall visualization", "OpenTelemetry export", "Per-trace cost", "Latency breakdown"], "Trace everything"],
  ["ipfence", "IPFence", "IP allowlisting for your LLM proxy", "Proxy is exposed to the internet. Someone discovers the endpoint. Runs up a $5,000 bill on your key.", "IP allowlist/denylist. CIDR ranges. Country-level blocking via GeoIP. Stop unauthorized access at the network level.", "🏰", "Security", ["IP allowlist/denylist", "CIDR range support", "Country blocking", "GeoIP integration", "Block logging", "Zero-config defaults"], "Lock the door"],
  ["embedcache", "EmbedCache", "Never compute the same embedding twice", "RAG pipeline re-embeds 10,000 documents every restart. $47 in embedding costs. Every. Single. Time.", "Content-hash caching for embedding requests. 100% deterministic = 100% cacheable. Track hit rate and dollar savings.", "💎", "Caching", ["Embedding-specific caching", "Content-hash dedup", "100% cache hit potential", "Cost savings tracking", "Hit rate dashboard", "Works with any embedding model"], "Cache embeddings"],
  ["anthrofit", "AnthroFit", "Use Claude with OpenAI SDKs", "App built on OpenAI SDK. Want to try Claude. Entire codebase needs rewriting for Anthropic's different API.", "Deep API translation: system messages, tool schemas, streaming format, response structure. Drop-in Claude support.", "🔌", "Compatibility", ["OpenAI→Anthropic translation", "System message handling", "Tool schema conversion", "Streaming format translation", "Response normalization", "Drop-in replacement"], "Switch to Claude"],
  ["alertpulse", "AlertPulse", "PagerDuty for your LLM stack", "OpenAI goes down at 3am. You find out from angry users at 9am.", "Configurable alerting rules. Error rates, latency spikes, cost thresholds. Slack, Discord, PagerDuty, email, webhooks.", "🚨", "Monitoring", ["Configurable alert rules", "Error rate thresholds", "Latency monitoring", "Cost threshold alerts", "Multi-channel notifications", "Alert history dashboard"], "Never miss an outage"],
  ["chatmem", "ChatMem", "Persistent memory without eating context", "Chatbot forgets everything after 8 messages. Stuffing full history burns $0.50 per request.", "Sliding window, summarization, importance-based memory strategies. Persistent across sessions. Context-window aware.", "🧠", "Memory", ["Sliding window memory", "Automatic summarization", "Importance scoring", "Cross-session persistence", "Token-aware management", "Memory strategy dashboard"], "Remember everything"],
  ["mockllm", "MockLLM", "Deterministic LLM responses for testing", "CI/CD hits real OpenAI API. Tests are slow, expensive, and randomly fail. $200/month in test API costs.", "Deterministic mock server. Prompt-matched fixtures. Regex matching. Delay simulation. Error injection. CI-friendly.", "🎭", "Testing", ["Deterministic responses", "Prompt-matched fixtures", "Regex matching", "Delay simulation", "Error injection", "CI/CD optimized"], "Test without APIs"],
  ["tenantwall", "TenantWall", "Per-tenant isolation for multi-tenant LLM apps", "Building SaaS with AI features. All tenants share one rate limit. One whale customer blocks everyone.", "Per-tenant rate limits, spend caps, model access, cache isolation. Tenant ID via header or key prefix.", "🏢", "Multi-Tenant", ["Per-tenant rate limits", "Per-tenant spend caps", "Model access control", "Cache isolation", "Tenant ID routing", "Per-tenant dashboard"], "Isolate tenants"],
  ["idlekill", "IdleKill", "Kill runaway LLM requests", "Agent loop burns $50 in 10 minutes. Streaming request hangs for 5 minutes doing nothing. Money on fire.", "Max duration, max tokens, max cost per request. Real-time streaming monitoring. Kill on threshold. Webhook alert.", "⏱️", "Cost Control", ["Per-request cost limits", "Max duration timeout", "Max token limits", "Streaming monitoring", "Auto-kill on threshold", "Webhook alerts on kill"], "Kill the runaway"],

  // Phase 3 P2 (21)
  ["agentguard", "AgentGuard", "Safety rails for autonomous AI agents", "Agent goes rogue. 200 API calls in 30 seconds. $500 gone before you notice.", "Per-session limits: max calls, cost, duration, allowed tools. Kill runaway sessions before they drain your budget.", "🛡️", "Agent Safety", ["Per-session call limits", "Per-session cost caps", "Duration limits", "Tool allowlists", "Auto-kill on breach", "Session analytics"], "Guard your agents"],
  ["codefence", "CodeFence", "Validate LLM-generated code before it runs", "LLM generates code with rm -rf /. Your eval() runs it. Production server gone.", "Scan for dangerous patterns: shell injection, file access, crypto mining. Syntax validation. Complexity scoring.", "🔒", "Code Safety", ["Dangerous pattern detection", "Shell injection scanning", "Syntax validation", "Complexity scoring", "Block/flag modes", "Custom pattern rules"], "Fence your code"],
  ["hallucicheck", "HalluciCheck", "Catch hallucinations before users do", "LLM invents a URL. User clicks it. Phishing site. You get the blame.", "Validate URLs, emails, citations in responses. Flag or retry when models invent non-existent references.", "🔍", "Quality", ["URL validation", "Email validation", "Citation checking", "Auto-retry on hallucination", "Confidence scoring", "Hallucination dashboard"], "Catch the lies"],
  ["tierdrop", "TierDrop", "Auto-downgrade models when burning cash", "CostCap blocks requests at budget. Users get errors. Revenue stops.", "Graceful degradation: GPT-4→GPT-3.5→GPT-4o-mini as spend approaches limits. Transparent to users.", "📉", "Cost Control", ["Cost-aware model selection", "Configurable tier chains", "Spend threshold triggers", "Transparent switching", "Per-tier tracking", "Gradual degradation"], "Degrade gracefully"],
  ["driftwatch", "DriftWatch", "Detect model behavior changes before users notice", "OpenAI silently updates GPT-4. Your carefully tuned prompts now produce garbage. Users complain for a week.", "Track latency, output patterns, quality scores per model over time. Alert when behavior drifts beyond thresholds.", "📊", "Monitoring", ["Behavioral baseline tracking", "Drift detection", "Per-model monitoring", "Threshold alerts", "Historical comparison", "Quality trend dashboard"], "Watch for drift"],
  ["feedbackloop", "FeedbackLoop", "Close the LLM improvement loop", "Users hit thumbs-down. The signal goes nowhere. Same bad responses forever.", "Capture ratings linked to request IDs. Track quality trends. Export worst-performing prompts for improvement.", "👍", "Feedback", ["Rating capture API", "Request-linked feedback", "Quality trend tracking", "Worst-prompt reports", "Export for fine-tuning", "Feedback dashboard"], "Close the loop"],
  ["abrouter", "ABRouter", "A/B test any LLM variable", "Want to test GPT-4 vs Claude. Manually split traffic. No statistics. Gut feeling decisions.", "Weighted traffic splits across any variable: models, temperatures, prompts. Statistical significance testing. Auto-promote winners.", "🧪", "Experimentation", ["Multi-variable A/B testing", "Weighted traffic splits", "Statistical significance", "Auto-promote winners", "Cost per variant", "Experiment dashboard"], "Test everything"],
  ["guardrail", "GuardRail", "Keep your LLM on-script", "Customer support bot gives medical advice. Code assistant writes poetry. Off-topic chaos.", "Define allowed/denied topics. Classify output. Block off-topic responses with custom fallback messages.", "🚧", "Safety", ["Topic allowlist/denylist", "Output classification", "Custom fallback messages", "Off-topic blocking", "Topic violation dashboard", "Configurable strictness"], "Stay on topic"],
  ["geminishim", "GeminiShim", "Tame Gemini behind a clean API", "Gemini safety filter blocks random requests. Token counts are wrong. Multimodal format is different.", "Handle safety filter blocks with auto-retry. Normalize token counts. OpenAI-compatible surface for Google's API.", "♊", "Compatibility", ["Safety filter retry", "Token normalization", "Multimodal translation", "OpenAI-compatible surface", "Error normalization", "Gemini-specific handling"], "Tame Gemini"],
  ["localsync", "LocalSync", "Blend local and cloud models seamlessly", "Ollama runs great locally. But sometimes it's down. Manual switching is a pain.", "Route to local Ollama when available. Auto-failover to cloud. Track cost savings from local inference.", "🏠", "Hybrid", ["Local-first routing", "Auto cloud failover", "Health checking", "Cost savings tracking", "Latency comparison", "Local/cloud dashboard"], "Go local first"],
  ["devproxy", "DevProxy", "Charles Proxy for LLM APIs", "LangChain sends something to OpenAI. What exactly? Headers mangled? Body correct? No visibility.", "Interactive debugging proxy. See every header, body, timing. Pause, inspect, edit requests mid-flight.", "🔧", "Debugging", ["Request/response inspection", "Header visibility", "Timing analysis", "Interactive debugging", "Pattern breakpoints", "Live request log"], "Debug everything"],
  ["promptslim", "PromptSlim", "Compress prompts 40-70% without losing meaning", "2K-token system prompt on every request. That's $0.06 per call just for the instructions.", "Remove redundant whitespace, filler words, articles. Configurable aggressiveness. See before/after token savings.", "✂️", "Cost Control", ["Automatic compression", "Filler word removal", "Whitespace optimization", "Configurable aggressiveness", "Before/after comparison", "Token savings tracking"], "Slim your prompts"],
  ["promptlint", "PromptLint", "Catch prompt anti-patterns", "Your prompt is redundant, injection-vulnerable, and wastes 400 tokens saying nothing. Nobody noticed.", "Static analysis for prompts. Detect redundancy, conflicts, injection patterns, excessive length. Score and suggest fixes.", "🔎", "Quality", ["Redundancy detection", "Conflict detection", "Injection vulnerability scan", "Length optimization", "Quality scoring", "Improvement suggestions"], "Lint your prompts"],
  ["approvalgate", "ApprovalGate", "Human approval for prompt changes", "Junior dev changes system prompt. Chatbot starts hallucinating. Deployed to production. No review process.", "Approval workflow for prompt modifications. Pending state. Approvers notified. Audit trail.", "✅", "Governance", ["Approval workflow", "Pending/approved/rejected states", "Approver notifications", "Audit trail", "Version tracking", "Dashboard management"], "Approve changes"],
  ["outputcap", "OutputCap", "Stop paying for unwanted verbosity", "Ask for one word. Get a 500-token essay. max_tokens cuts mid-sentence.", "Cap output at natural sentence boundaries. Detect completion points. No more paying for essays you don't need.", "📏", "Cost Control", ["Natural boundary detection", "Sentence-level capping", "Configurable max length", "Token savings tracking", "Mid-sentence prevention", "Smart truncation"], "Cap the output"],
  ["agegate", "AgeGate", "Child safety for LLM apps", "App has users under 18. No content filtering. COPPA violation waiting to happen.", "Age tier config. Inject age-appropriate system prompts. Filter adult content, violence, self-harm references.", "👶", "Safety", ["Age tier configuration", "Age-appropriate prompts", "Content filtering", "COPPA/KOSA ready", "Violation logging", "Per-tier policies"], "Protect kids"],
  ["voicebridge", "VoiceBridge", "LLM middleware for voice pipelines", "LLM returns markdown, URLs, code blocks. TTS reads them literally. Users hear 'asterisk asterisk bold text asterisk asterisk.'", "Strip markdown, convert lists to prose, remove code blocks. Enforce max length. TTFB tracking for voice latency.", "🎙️", "Voice", ["Markdown stripping", "List-to-prose conversion", "Code block removal", "Max length enforcement", "TTFB tracking", "Voice-optimized output"], "Voice-ready output"],
  ["imageproxy", "ImageProxy", "Proxy magic for image generation", "DALL-E costs add up. Same prompt generates same image. No caching. No failover.", "Cost tracking, caching, and failover for DALL-E and other image generation APIs. Prompt-hash caching.", "🎨", "Multimodal", ["Image gen cost tracking", "Prompt-hash caching", "Provider failover", "Per-image cost tracking", "Generation dashboard", "Multi-provider support"], "Proxy your images"],
  ["langbridge", "LangBridge", "Multilingual LLM in one config line", "App serves 20 languages. System prompt is English. Ad-hoc translation in every endpoint.", "Auto-detect language. Translate to English for model. Translate response back. Cache translations. Seamless multilingual.", "🌐", "i18n", ["Language detection", "Auto-translation", "Response translation", "Translation caching", "20+ languages", "Transparent to apps"], "Go multilingual"],
  ["contextwindow", "ContextWindow", "Visual context window debugger", "Bad responses because context is full of junk. Can't see what's eating your tokens.", "Visualize token allocation by message role. See what's eating your context window. Optimization recommendations.", "🪟", "Debugging", ["Token breakdown by role", "Visual allocation chart", "Truncation highlighting", "Optimization recommendations", "Per-section analysis", "Context usage history"], "See your context"],
  ["regionroute", "RegionRoute", "Data residency routing for GDPR", "GDPR requires EU data stays in EU. Your proxy sends everything to US endpoints.", "Route by header, tenant, or IP geolocation. Map regions to provider endpoints. ComplianceLog integration.", "🌍", "Compliance", ["Geographic routing", "EU data residency", "Tenant-based routing", "IP geolocation", "Provider endpoint mapping", "GDPR compliance ready"], "Route by region"],

  // Phase 3 P3 (15)
  ["chainforge", "ChainForge", "Multi-step LLM workflows as YAML", "Extract→analyze→summarize→format. Coded in Python with 200 lines of glue code per pipeline.", "Define pipelines in YAML. Steps with data passing. Conditional branching. Parallel execution. Cost tracking per pipeline.", "⛓️", "Workflow", ["YAML pipeline definitions", "Step data passing", "Conditional branching", "Parallel execution", "Per-pipeline cost tracking", "Pipeline dashboard"], "Chain your LLMs"],
  ["cronllm", "CronLLM", "Your AI cron job runner", "Daily summaries need a separate cron script. Weekly reports need another. Manual cron + curl for each.", "Define scheduled prompts in YAML. Daily, weekly, monthly. Runs through full proxy chain. Output to webhook or file.", "⏰", "Automation", ["YAML job definitions", "Cron scheduling", "Full proxy chain", "Output destinations", "Job history tracking", "Failure alerting"], "Schedule your AI"],
  ["webhookrelay", "WebhookRelay", "Webhook → LLM → Action in one config", "GitHub issue → summarize → Slack. Currently: Lambda + SQS + custom code. 3 services for one pipe.", "Inbound webhook endpoint. Extract data. Build prompt. Call LLM. Send result to destination. All in YAML.", "🔗", "Integration", ["Inbound webhook endpoint", "Data extraction", "Prompt templating", "LLM call routing", "Output destinations", "Event history"], "Connect everything"],
  ["billsync", "BillSync", "Per-customer LLM invoices automatically", "Reselling LLM access. Tracking per-customer usage in a spreadsheet. Invoicing manually.", "Track usage per tenant. Apply markup percentage. Generate invoice data. Stripe-compatible usage records.", "💰", "Billing", ["Per-tenant tracking", "Configurable markup", "Invoice generation", "Stripe compatibility", "Billing period management", "Revenue dashboard"], "Bill your customers"],
  ["whitelabel", "WhiteLabel", "Your brand on Stockyard's engine", "Selling LLM infrastructure but dashboards say 'Stockyard' everywhere. Not your brand.", "Custom logo, colors, brand name, CSS. Your branding on the entire dashboard. Suite tier feature.", "🏷️", "Branding", ["Custom logo", "Brand colors", "Custom CSS", "Domain override", "Brand name replacement", "Dashboard theming"], "Make it yours"],
  ["trainexport", "TrainExport", "Export conversations as training data", "100K logged requests = valuable fine-tuning data stuck in SQLite. No way to get it out.", "Collect input/output pairs. Export as OpenAI JSONL, Anthropic, Alpaca format. Quality filters. PII redaction.", "📤", "Data", ["Pair collection", "OpenAI JSONL export", "Anthropic format export", "Quality filtering", "PII redaction", "Configurable formats"], "Export training data"],
  ["synthgen", "SynthGen", "Generate synthetic training data at scale", "Need 10K training examples. Manually writing them takes weeks.", "Templates + seed examples → synthetic data at scale. Quality-checked through your proxy chain. Deduplicated.", "🧬", "Data", ["Template-based generation", "Seed examples", "Batch processing", "Quality checking", "Deduplication", "Scale to millions"], "Generate data"],
  ["diffprompt", "DiffPrompt", "Git-style diff for prompt changes", "System prompt changed. When? By who? What changed? No visibility into prompt modifications.", "Hash-based change detection. Track every system prompt modification. Per-model change history.", "📝", "Versioning", ["SHA256 change detection", "Per-model tracking", "Change history", "Diff visualization", "Alert on changes", "Audit trail"], "Diff your prompts"],
  ["llmbench", "LLMBench", "Benchmark models on YOUR workload", "MMLU scores are meaningless for your use case. Which model is actually best for YOUR prompts?", "Per-model stats from real traffic: latency, cost, quality, tokens. Compare on your actual workload, not benchmarks.", "🏋️", "Benchmarking", ["Real-workload benchmarks", "Per-model comparison", "Latency tracking", "Cost comparison", "Quality scoring", "Benchmark reports"], "Benchmark realistically"],
  ["maskmode", "MaskMode", "Demo mode with realistic fake data", "Sales demo needs data but can't show real customer PII. [REDACTED] looks terrible.", "Replace names, emails, phones with realistic fakes. Consistent within session. Perfect for demos and screenshots.", "🎭", "Privacy", ["Realistic fake names", "Fake emails/phones", "Session consistency", "Demo-ready output", "Pattern matching", "Configurable masks"], "Demo safely"],
  ["tokenmarket", "TokenMarket", "Dynamic budget reallocation across teams", "Engineering burned their LLM budget. Marketing barely uses theirs. No way to move money around.", "Pool-based budgets. Teams request capacity. Auto-rebalance. Priority queuing for high-value requests.", "🏪", "Budgets", ["Budget pools", "Dynamic reallocation", "Priority queuing", "Team-based budgets", "Auto-rebalance", "Spending dashboard"], "Trade capacity"],
  ["llmsync", "LLMSync", "Sync config across environments", "Different configs for dev/staging/prod. They drifted apart months ago. Manual tracking in a doc.", "Environment hierarchy. Config inheritance with overrides. Diff between environments. Promote. Rollback. Git-friendly.", "🔄", "DevOps", ["Environment hierarchy", "Config inheritance", "Diff command", "Promote/rollback", "Git-friendly YAML", "Override management"], "Sync your config"],
  ["clustermode", "ClusterMode", "Scale beyond one instance", "Traffic outgrew single instance. SQLite is single-writer. Can't horizontally scale.", "Multi-instance coordination. Leader-follower with shared cache. Gossip protocol. Scale horizontally.", "🏗️", "Infrastructure", ["Multi-instance mode", "Leader-follower", "Shared cache", "Gossip protocol", "Horizontal scaling", "Node health monitoring"], "Go multi-instance"],
  ["encryptvault", "EncryptVault", "Encrypt sensitive LLM payloads", "Healthcare app. Patient data in prompts. Stored in plaintext SQLite. HIPAA violation.", "AES-GCM encryption for sensitive fields. Customer-managed keys (BYOK). Decrypt on read. Compliance ready.", "🔐", "Security", ["AES-GCM encryption", "Field-level encryption", "Customer-managed keys", "BYOK support", "Decrypt on read", "HIPAA/SOC2 ready"], "Encrypt everything"],
  ["mirrortest", "MirrorTest", "Shadow test against production traffic", "Want to evaluate Claude on real queries. But can't risk sending bad responses to users.", "Shadow traffic to a second model. Compare quality, latency, cost. Zero user impact. Real-world evaluation.", "🪞", "Testing", ["Shadow traffic routing", "Dual-model comparison", "Quality comparison", "Latency comparison", "Zero user impact", "Configurable sample rate"], "Shadow test safely"],

  // Phase 4 (57) — abbreviated, key conversion elements only
  ["extractml", "ExtractML", "Turn prose into structured data", "LLM returns paragraphs when you wanted JSON fields.", "Force extraction from free-text into your schema.", "🧲", "Structured Data", ["Schema-based extraction", "Auto-inject extraction", "Pattern caching", "Prose→JSON conversion", "Configurable schemas", "Extraction dashboard"], "Extract structure"],
  ["tableforge", "TableForge", "LLM-powered CSV with type validation", "Model makes tables with missing columns and wrong types.", "Validate columns, types, completeness. Auto-repair. Export.", "📊", "Structured Data", ["Column validation", "Type checking", "Auto-repair", "CSV/JSON export", "Completeness scoring", "Table dashboard"], "Forge better tables"],
  ["toolrouter", "ToolRouter", "Manage and version LLM function calls", "30 tools registered. No versioning. No analytics.", "Versioned schemas. Route calls. Shadow-test. Usage analytics.", "🔀", "Function Calling", ["Tool versioning", "Call routing", "Shadow testing", "Usage analytics", "Schema registry", "Tool dashboard"], "Route your tools"],
  ["toolshield", "ToolShield", "Sandbox LLM tool calls", "LLM calls delete_all_users() with bad args.", "Validate args. Per-tool permissions. Rate limits per tool.", "🛡️", "Function Calling", ["Argument validation", "Per-tool permissions", "Rate limits", "Dangerous call blocking", "Call audit log", "Tool analytics"], "Shield your tools"],
  ["toolmock", "ToolMock", "Fake tool responses for testing", "Testing tool-use agents requires real external services.", "Canned responses. Error simulation. Timeout simulation.", "🃏", "Testing", ["Canned responses", "Error simulation", "Timeout simulation", "Pattern matching", "Test fixtures", "CI-friendly"], "Mock your tools"],
  ["authgate", "AuthGate", "API key management for YOUR users", "Building API product. Need to issue keys to customers.", "Issue/revoke keys. Per-key limits. Usage tracking.", "🔑", "Auth", ["Key issuance", "Key revocation", "Per-key limits", "Usage tracking", "Key scoping", "Customer dashboard"], "Issue API keys"],
  ["scopeguard", "ScopeGuard", "Fine-grained permissions per key", "Intern's key accesses GPT-4. Free users hit embeddings.", "Role-based access. Map keys to allowed models and endpoints.", "🎯", "Auth", ["Role definitions", "Model restrictions", "Endpoint permissions", "Token budgets", "Denial audit log", "Permission dashboard"], "Scope your keys"],
  ["visionproxy", "VisionProxy", "Proxy for vision/image APIs", "GPT-4V calls are expensive. No caching. No failover.", "Image-aware caching, cost tracking, failover.", "👁️", "Multimodal", ["Image hashing", "Vision caching", "Per-image costs", "Provider failover", "Size optimization", "Vision dashboard"], "Proxy your vision"],
  ["audioproxy", "AudioProxy", "Proxy for STT/TTS APIs", "Whisper and ElevenLabs need same infra.", "Cache TTS. Track per-minute costs. Failover.", "🔊", "Multimodal", ["TTS caching", "Per-minute costs", "Provider failover", "STT routing", "Audio analytics", "Cost savings tracking"], "Proxy your audio"],
  ["docparse", "DocParse", "Preprocess documents for LLMs", "PDFs need extraction and chunking before LLM.", "Extract text. Smart chunking. Clean artifacts.", "📄", "Multimodal", ["PDF extraction", "Smart chunking", "Artifact cleaning", "Multiple formats", "Chunk optimization", "Processing dashboard"], "Parse your docs"],
  ["framegrab", "FrameGrab", "Video frames through vision LLMs", "Video analysis is expensive per frame.", "Smart frame selection. Batch processing. Scene detection.", "🎬", "Multimodal", ["Scene detection", "Smart frame selection", "Batch processing", "Cost per frame", "Cache analyses", "Frame dashboard"], "Grab key frames"],
  ["sessionstore", "SessionStore", "Managed conversation sessions", "Every chatbot rebuilds session management.", "Create/resume/list/delete. Full history. Metadata.", "💬", "Sessions", ["Session CRUD", "Full history", "Metadata support", "Concurrent limits", "Session sharing", "Export capability"], "Manage sessions"],
  ["convofork", "ConvoFork", "Branch conversations", "What if I'd asked differently? Linear only.", "Fork at any message. Independent branches.", "🌿", "Sessions", ["Branch at any point", "Independent histories", "Tree visualization", "Branch comparison", "Merge capability", "Fork analytics"], "Fork conversations"],
  ["slotfill", "SlotFill", "Form-filling conversation engine", "Multi-turn data collection coded from scratch.", "Define slots. Track filled/missing. Reprompt.", "📋", "Sessions", ["Slot definitions", "Type validation", "Auto-reprompting", "Completion tracking", "Funnel analytics", "Export filled data"], "Fill the slots"],
  ["semanticcache", "SemanticCache", "Cache similar prompts, not just identical", "'Weather NYC' and 'weather New York City' = cache miss.", "Similarity-based matching. 10x hit rate.", "🧠", "Caching", ["Semantic similarity matching", "Configurable threshold", "10x hit rate", "Embedding-based", "Savings tracking", "Hit rate dashboard"], "Cache semantically"],
  ["partialcache", "PartialCache", "Cache prompt prefixes", "3K system prompt processed 1000x = 3M wasted tokens.", "Detect static prefix. Use native prefix caching.", "🧩", "Caching", ["Prefix detection", "Native prefix caching", "Token savings", "Auto-detection", "Provider-aware", "Cache analytics"], "Cache your prefix"],
  ["streamcache", "StreamCache", "Cache streaming with realistic timing", "Cached response arrives instantly. Looks wrong in chat UI.", "Store original timing. Replay with original pacing.", "📺", "Caching", ["Timing preservation", "SSE replay", "Realistic pacing", "Instant mode option", "Stream storage", "Cache analytics"], "Stream from cache"],
  ["promptchain", "PromptChain", "Composable prompt blocks", "2K system prompt copy-pasted across 15 products.", "Define blocks. Compose: [tone + format + domain].", "🔗", "Prompt Management", ["Reusable blocks", "Composition syntax", "Auto-update", "Block versioning", "Template variables", "Block analytics"], "Compose your prompts"],
  ["promptfuzz", "PromptFuzz", "Fuzz-test your prompts", "Tested with 10 normal inputs. Adversarial? Untested.", "Generate adversarial inputs. Score failures.", "🐛", "Testing", ["Adversarial generation", "Multi-language inputs", "Edge cases", "Failure scoring", "Report generation", "CI integration"], "Fuzz your prompts"],
  ["promptmarket", "PromptMarket", "Community prompt library", "Everyone reinvents the same system prompts.", "Publish, browse, rate, fork community prompts.", "🏪", "Community", ["Publish prompts", "Browse + search", "Rating system", "Fork + customize", "Usage tracking", "Free adoption driver"], "Share prompts"],
  ["costpredict", "CostPredict", "Predict cost BEFORE sending", "About to send 50K tokens to GPT-4. Cost unknown.", "Count input tokens. Estimate output. X-Estimated-Cost header.", "🔮", "Cost Intel", ["Pre-send estimation", "Input token counting", "Output estimation", "Cost header", "Optional blocking", "Prediction accuracy tracking"], "Predict your costs"],
  ["costmap", "CostMap", "Multi-dimensional cost attribution", "LLM bill is $2K/mo. Which feature? Which user?", "Tag + aggregate. Interactive drill-down dashboard.", "🗺️", "Cost Intel", ["Dimension tagging", "Multi-level drill-down", "Interactive dashboard", "Export to BI tools", "Trend analysis", "Attribution reports"], "Map your costs"],
  ["spotprice", "SpotPrice", "Route to the cheapest model", "Prices change. New models launch. Overpaying.", "Live pricing DB. Route to cheapest meeting quality.", "💱", "Cost Intel", ["Live pricing database", "Quality-aware routing", "Cost optimization", "Price change tracking", "Savings reports", "Provider comparison"], "Find the best price"],
  ["loadforge", "LoadForge", "Load test your LLM stack", "How many concurrent requests before degradation?", "Measure TTFT, TPS, p50/p95/p99, errors.", "⚡", "Testing", ["Load profile definition", "TTFT measurement", "TPS tracking", "Percentile latency", "Error rate tracking", "Capacity reports"], "Load test it"],
  ["snapshottest", "SnapshotTest", "Snapshot testing for LLM outputs", "Prompt change breaks output structure. No detection.", "Record baselines. Semantic diff. CI-friendly.", "📸", "Testing", ["Baseline recording", "Semantic diffing", "Configurable threshold", "CI integration", "Snapshot updates", "Regression reports"], "Snapshot your outputs"],
  ["chaosllm", "ChaosLLM", "Chaos engineering for LLMs", "What happens when OpenAI returns 500s?", "Inject 429s, timeouts, malformed JSON, truncated streams.", "💥", "Testing", ["Fault injection", "429 simulation", "Timeout injection", "Malformed responses", "Truncated streams", "Configurable rates"], "Break things safely"],
  ["datamap", "DataMap", "GDPR data flow mapping", "GDPR Article 30 requires data flow records.", "Auto-classify. Map source→proxy→provider→storage.", "🗃️", "Compliance", ["Auto-classification", "Flow mapping", "GDPR Article 30", "Record generation", "Export formats", "Flow visualization"], "Map your data"],
  ["consentgate", "ConsentGate", "Consent management for AI", "EU AI Act requires informed consent for AI.", "Check consent per user. Block non-consented.", "✋", "Compliance", ["Per-user consent", "Block non-consented", "Consent timestamps", "Withdrawal support", "Audit logging", "Consent dashboard"], "Manage consent"],
  ["retentionwipe", "RetentionWipe", "Automated data retention", "GDPR right to erasure. Logs kept forever.", "Retention periods. Auto-purge. Per-user deletion.", "🧹", "Compliance", ["Retention policies", "Auto-purge", "Per-user deletion", "Deletion certificates", "Configurable periods", "Compliance reports"], "Wipe on schedule"],
  ["policyengine", "PolicyEngine", "AI governance as code", "AI policies are documents nobody reads.", "YAML rules → enforceable middleware. Audit log.", "📜", "Compliance", ["YAML policy rules", "Middleware compilation", "Audit logging", "Compliance rates", "Policy versioning", "Enforcement dashboard"], "Enforce your policies"],
  ["streamsplit", "StreamSplit", "Fork streams to multiple destinations", "Want stream to user AND logger AND quality checker.", "Tee SSE to all destinations. Zero primary latency.", "🔱", "Streaming", ["Stream forking", "Multi-destination", "Zero-latency primary", "Configurable targets", "Per-target analytics", "Webhook destinations"], "Split your streams"],
  ["streamthrottle", "StreamThrottle", "Control streaming speed", "Claude streams too fast for UI rendering.", "Max tokens/sec. Buffer fast streams. Per-client.", "🚦", "Streaming", ["Speed control", "Token rate limiting", "Per-client config", "Buffer management", "UX optimization", "Throttle analytics"], "Throttle your streams"],
  ["streamtransform", "StreamTransform", "Transform streams mid-flight", "Need to strip markdown from stream before voice.", "Pipeline on chunks: strip, redact, translate.", "🔄", "Streaming", ["Chunk transformation", "Markdown stripping", "PII redaction", "Real-time translation", "Pipeline chaining", "Latency tracking"], "Transform in-flight"],
  ["modelalias", "ModelAlias", "Abstract away model names", "Code references gpt-4-0613. Gets deprecated. Update 50 configs.", "Aliases: fast→gpt-4o-mini. Change one mapping.", "🏷️", "Routing", ["Name abstraction", "One-place updates", "Alias mapping", "Zero-downtime changes", "Version management", "Usage per alias"], "Alias your models"],
  ["paramnorm", "ParamNorm", "Normalize params across providers", "temperature=0.7 means different things on different models.", "Calibration profiles. Normalized → model-specific.", "⚖️", "Compatibility", ["Parameter calibration", "Cross-model normalization", "Per-model profiles", "Consistent behavior", "Profile management", "Calibration dashboard"], "Normalize params"],
  ["quotasync", "QuotaSync", "Track provider rate limits", "Don't know how much of OpenAI's 10K RPM you've used.", "Parse rate limit headers. Alert near limits.", "📈", "Monitoring", ["Header parsing", "Real-time tracking", "Near-limit alerts", "Per-model quotas", "Usage forecasting", "Quota dashboard"], "Track your quotas"],
  ["errornorm", "ErrorNorm", "Normalize errors across providers", "OpenAI, Anthropic, Gemini all return different errors.", "Single schema: code, message, provider, retry_after.", "⚠️", "Compatibility", ["Error normalization", "Single schema", "Retry guidance", "Provider tagging", "Error analytics", "Pattern detection"], "Normalize your errors"],
  ["cohorttrack", "CohortTrack", "User cohort analytics", "Are new users more engaged than old users?", "Cohorts by signup, plan, feature. Retention curves.", "👥", "Analytics", ["Cohort definition", "Retention tracking", "Cost per cohort", "Feature adoption", "BI export", "Cohort dashboard"], "Track your cohorts"],
  ["promptrank", "PromptRank", "Rank prompts by ROI", "50 templates. Some expensive and bad. No comparison.", "Per template: cost, quality, latency, volume. Leaderboard.", "🏆", "Analytics", ["ROI calculation", "Quality scoring", "Cost tracking", "Volume analysis", "Prompt leaderboard", "Optimization suggestions"], "Rank your prompts"],
  ["anomalyradar", "AnomalyRadar", "ML-powered anomaly detection", "AlertPulse thresholds are static. You don't know normal.", "Statistical baselines. Z-score detection. Auto-adjusting.", "📡", "Analytics", ["Baseline learning", "Z-score detection", "Auto-adjusting thresholds", "Multi-metric monitoring", "Anomaly alerts", "Pattern visualization"], "Detect anomalies"],
  ["envsync", "EnvSync", "Sync configs + secrets", "LLMSync handles config. Secrets are separate.", "Push/promote/diff with encrypted secrets.", "🔐", "DevOps", ["Secret encryption", "Environment sync", "Push/promote", "Diff command", "Rollback support", "Validation checks"], "Sync your secrets"],
  ["proxylog", "ProxyLog", "Log every proxy decision", "Why did it pick provider B? Why cache miss?", "Per-middleware decision logs. X-Proxy-Trace header.", "📋", "Debugging", ["Decision logging", "Per-middleware trace", "X-Proxy-Trace header", "Decision analysis", "Pattern detection", "Trace dashboard"], "Log decisions"],
  ["clidash", "CliDash", "Terminal dashboard for your LLM stack", "Opening browser breaks flow. Want terminal monitoring.", "Real-time TUI: req/sec, models, cache, spend, errors.", "🖥️", "DevOps", ["Terminal UI", "Real-time metrics", "Keyboard navigation", "SSH accessible", "Configurable views", "Low resource usage"], "Monitor in terminal"],
  ["embedrouter", "EmbedRouter", "Smart embedding request routing", "RAG sends thousands of individual embedding requests.", "Batch over 50ms. Deduplicate. Route by content type.", "🔀", "Embeddings", ["Batching window", "Deduplication", "Content-type routing", "Parallel processing", "Cost optimization", "Routing analytics"], "Route embeddings"],
  ["finetunetrack", "FineTuneTrack", "Monitor fine-tuned model performance", "Fine-tuned 3 months ago. Still good? Data drifted?", "Periodic eval suite. Track scores. Compare to base.", "📉", "Monitoring", ["Periodic evaluation", "Score tracking", "Base model comparison", "Drift detection", "Alert on degradation", "Performance dashboard"], "Track your fine-tunes"],
  ["agentreplay", "AgentReplay", "Replay agent sessions step-by-step", "Agent did something weird. Can't reproduce.", "Step-by-step playback. What-if mode. Export as tests.", "🎬", "Debugging", ["Step-by-step playback", "What-if mode", "Test case export", "Session search", "Decision tree view", "Replay dashboard"], "Replay your agents"],
  ["summarizegate", "SummarizeGate", "Auto-summarize long contexts", "RAG retrieves 20 chunks. Only 5 relevant.", "Score relevance. Summarize low-relevance. Save tokens.", "📝", "Optimization", ["Relevance scoring", "Selective summarization", "Token savings", "Configurable threshold", "Quality preservation", "Savings tracking"], "Summarize smartly"],
  ["codelang", "CodeLang", "Language-aware code validation", "CodeFence uses regex. Misses real syntax errors.", "Tree-sitter parsing. Syntax validation per language.", "💻", "Code Safety", ["Tree-sitter parsing", "Multi-language support", "Syntax validation", "Undefined reference detection", "Pattern matching", "Validation dashboard"], "Validate with parsers"],
  ["personaswitch", "PersonaSwitch", "Hot-swap AI personalities", "Multiple personas = multiple system prompts in code.", "Define personas. Route by header/key/segment.", "🎭", "Customization", ["Persona definitions", "Header-based routing", "Temperature per persona", "Format rules", "Segment targeting", "Persona analytics"], "Switch personas"],
  ["warmpool", "WarmPool", "Pre-warm model connections", "First request per provider is slower. Cold start.", "Persistent connections. Health checks. Keep-alive.", "🔥", "Performance", ["Connection pre-warming", "Health checks", "Keep-alive management", "Ollama keep-warm", "Latency reduction", "Connection pool dashboard"], "Warm your connections"],
  ["edgecache", "EdgeCache", "CDN-like caching for LLM responses", "Users are global. Cache is local.", "Distribute cache. Geographic hit rates.", "🌐", "Caching", ["Distributed cache", "Multi-instance replication", "Geographic hit rates", "LiteFS support", "Cache coherence", "Global analytics"], "Cache at the edge"],
  ["queuepriority", "QueuePriority", "Priority queues — VIPs first", "BatchQueue is FIFO. Enterprise should jump ahead.", "Priority levels per key/tenant. Reserved capacity.", "👑", "Infrastructure", ["Priority levels", "Reserved capacity", "SLA tracking", "Per-key priority", "Queue analytics", "Starvation prevention"], "Prioritize traffic"],
  ["geoprice", "GeoPrice", "Regional pricing by purchasing power", "$59/mo is nothing in SF, significant in Lagos.", "PPP-adjusted pricing. Anti-VPN. Regional analytics.", "💱", "Billing", ["PPP adjustment", "Regional pricing", "Anti-VPN detection", "Revenue by region", "Dynamic pricing", "Equity analytics"], "Price fairly"],
  ["tokenauction", "TokenAuction", "Dynamic pricing based on demand", "Fixed pricing doesn't match variable costs.", "Demand-based pricing. Time-of-day. Surge pricing.", "🏷️", "Billing", ["Demand monitoring", "Dynamic pricing", "Time-of-day rates", "Surge pricing", "Cost tracking", "Pricing dashboard"], "Price dynamically"],
  ["canarydeploy", "CanaryDeploy", "Canary deployments for model changes", "Want to switch models 5%→25%→100%.", "Gradual rollout. Auto-promote if quality holds.", "🐤", "Deployment", ["Gradual rollout", "Quality monitoring", "Auto-promote", "Auto-rollback", "Traffic splitting", "Canary dashboard"], "Deploy carefully"],
  ["playbackstudio", "PlaybackStudio", "Interactive playground for logged interactions", "Thousands of logs. Finding interesting ones is a haystack.", "Advanced filters. Side-by-side. Bulk actions.", "🎪", "Analytics", ["Advanced filtering", "Conversation threads", "Side-by-side comparison", "Bulk actions", "Content search", "Interactive explorer"], "Explore your logs"],
  ["webhookforge", "WebhookForge", "Visual webhook→LLM→action builder", "WebhookRelay is config-only. Want visual flows.", "Visual builder. Multi-step. Condition branches. History.", "⚒️", "Automation", ["Visual flow builder", "Multi-step pipelines", "Conditional branching", "Execution history", "Error handling", "Template library"], "Build visually"],
];

// ─── HTML Template ────────────────────────────────────────────────────
function generatePage(key, displayName, tagline, painPoint, solution, icon, category, features, cta) {
  const pricingNote = key === "stockyard"
    ? `<div class="pricing-highlight"><span class="price">$59</span>/month for all 125 products</div>`
    : `<div class="pricing-grid">
        <div class="tier"><div class="tier-name">Free</div><div class="tier-price">$0</div><div class="tier-desc">Limited usage</div></div>
        <div class="tier featured"><div class="tier-name">Pro</div><div class="tier-price">$29</div><div class="tier-desc">Full features</div></div>
        <div class="tier"><div class="tier-name">Team</div><div class="tier-price">$79</div><div class="tier-desc">Multi-user</div></div>
      </div>
      <p class="suite-nudge">Or get <strong>all 125 products</strong> in the <a href="/products/stockyard/">Stockyard Suite</a> for <strong>$59/mo</strong></p>`;

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>${displayName} — ${tagline} | Stockyard</title>
<meta name="description" content="${displayName}: ${tagline}. ${solution.slice(0, 120)}">
<meta property="og:title" content="${displayName} — ${tagline}">
<meta property="og:description" content="${solution.slice(0, 200)}">
<meta property="og:type" content="website">
<meta property="og:url" content="https://stockyard.dev/products/${key}/">
<meta name="twitter:card" content="summary_large_image">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;
  --rust:#c45d2c;--rust-light:#e8753a;--rust-dark:#8b3d1a;
  --leather:#a0845c;--leather-light:#c4a87a;
  --cream:#f0e6d3;--cream-dim:#bfb5a3;
  --gold:#d4a843;
  --font-serif:'Libre Baskerville',Georgia,serif;
  --font-mono:'JetBrains Mono',monospace;
}
body{background:var(--bg);color:var(--cream);font-family:var(--font-serif);line-height:1.7;overflow-x:hidden}
a{color:var(--rust-light);text-decoration:none}
a:hover{color:var(--gold)}

/* Nav */
.nav{padding:1rem 2rem;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--bg3)}
.nav-brand{font-family:var(--font-mono);font-size:0.9rem;color:var(--leather-light);letter-spacing:2px;text-transform:uppercase}
.nav-links{display:flex;gap:1.5rem;font-size:0.85rem;font-family:var(--font-mono)}
.nav-links a{color:var(--cream-dim)}
.nav-links a:hover{color:var(--rust-light)}

/* Hero */
.hero{max-width:800px;margin:0 auto;padding:6rem 2rem 4rem;text-align:center}
.hero-mark{font-family:var(--font-mono);font-size:0.75rem;color:var(--leather);letter-spacing:4px;margin-bottom:1.5rem;display:block}
.hero-mark::before{content:'';display:block;width:40px;height:1px;background:var(--rust);margin:0 auto 1rem}
.hero-category{font-family:var(--font-mono);font-size:0.75rem;text-transform:uppercase;letter-spacing:3px;color:var(--rust);margin-bottom:1rem}
.hero h1{font-size:clamp(2rem,5vw,3.2rem);line-height:1.2;margin-bottom:1rem;color:var(--cream)}
.hero h1 .accent{color:var(--rust-light)}
.hero-tagline{font-size:1.15rem;color:var(--cream-dim);font-style:italic;margin-bottom:2.5rem;max-width:600px;margin-left:auto;margin-right:auto}

/* CTA */
.cta-group{display:flex;gap:1rem;justify-content:center;flex-wrap:wrap;margin-bottom:1rem}
.btn{font-family:var(--font-mono);font-size:0.85rem;padding:0.8rem 2rem;border:none;cursor:pointer;transition:all 0.2s;text-decoration:none;display:inline-block}
.btn-primary{background:var(--rust);color:var(--cream);border:2px solid var(--rust)}
.btn-primary:hover{background:var(--rust-light);border-color:var(--rust-light);color:#fff}
.btn-secondary{background:transparent;color:var(--cream);border:2px solid var(--leather)}
.btn-secondary:hover{border-color:var(--cream);color:var(--cream)}
.install-cmd{font-family:var(--font-mono);font-size:0.8rem;color:var(--leather-light);background:var(--bg2);padding:0.5rem 1.5rem;border-radius:4px;margin-top:1rem;display:inline-block;border:1px solid var(--bg3)}

/* Pain section */
.pain{background:var(--bg2);padding:4rem 2rem;border-top:1px solid var(--bg3);border-bottom:1px solid var(--bg3)}
.pain-inner{max-width:700px;margin:0 auto}
.pain-label{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:3px;color:var(--rust);margin-bottom:1rem}
.pain-text{font-size:1.1rem;color:var(--cream-dim);line-height:1.8;font-style:italic}
.pain-text::before{content:'"';font-size:3rem;color:var(--rust-dark);line-height:0;vertical-align:-0.5rem;margin-right:0.2rem}

/* Solution */
.solution{padding:4rem 2rem;max-width:700px;margin:0 auto}
.solution-label{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:3px;color:var(--gold);margin-bottom:1rem}
.solution-text{font-size:1.05rem;color:var(--cream);line-height:1.8}

/* Features */
.features{padding:4rem 2rem;background:var(--bg2);border-top:1px solid var(--bg3);border-bottom:1px solid var(--bg3)}
.features-inner{max-width:800px;margin:0 auto}
.features-label{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:3px;color:var(--leather-light);margin-bottom:2rem;text-align:center}
.features-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:1.5rem}
.feature-item{padding:1.2rem;border:1px solid var(--bg3);background:var(--bg)}
.feature-item::before{content:'—';color:var(--rust);font-family:var(--font-mono);margin-right:0.5rem}
.feature-item span{font-size:0.9rem;color:var(--cream-dim)}

/* Quickstart */
.quickstart{padding:4rem 2rem;max-width:700px;margin:0 auto}
.quickstart-label{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:3px;color:var(--rust);margin-bottom:1.5rem;text-align:center}
.code-block{background:var(--bg2);border:1px solid var(--bg3);padding:1.5rem;font-family:var(--font-mono);font-size:0.8rem;color:var(--leather-light);overflow-x:auto;line-height:1.8}
.code-block .comment{color:#5a5040}
.code-block .cmd{color:var(--cream)}

/* Pricing */
.pricing{padding:4rem 2rem;text-align:center;background:var(--bg2);border-top:1px solid var(--bg3);border-bottom:1px solid var(--bg3)}
.pricing-label{font-family:var(--font-mono);font-size:0.7rem;text-transform:uppercase;letter-spacing:3px;color:var(--gold);margin-bottom:2rem}
.pricing-grid{display:flex;gap:1.5rem;justify-content:center;flex-wrap:wrap;margin-bottom:1.5rem}
.tier{padding:2rem;border:1px solid var(--bg3);background:var(--bg);min-width:160px}
.tier.featured{border-color:var(--rust);position:relative}
.tier.featured::after{content:'Popular';position:absolute;top:-0.6rem;right:1rem;background:var(--rust);color:var(--cream);font-family:var(--font-mono);font-size:0.65rem;padding:0.2rem 0.6rem;letter-spacing:1px;text-transform:uppercase}
.tier-name{font-family:var(--font-mono);font-size:0.75rem;text-transform:uppercase;letter-spacing:2px;color:var(--leather-light);margin-bottom:0.5rem}
.tier-price{font-size:2rem;font-weight:700;color:var(--cream);margin-bottom:0.3rem}
.tier-price::before{content:'$';font-size:1rem;vertical-align:super;margin-right:2px}
.tier-desc{font-size:0.8rem;color:var(--cream-dim)}
.suite-nudge{font-size:0.85rem;color:var(--cream-dim);max-width:500px;margin:0 auto}
.suite-nudge a{color:var(--gold)}
.pricing-highlight{margin-bottom:1.5rem}
.pricing-highlight .price{font-size:3rem;font-weight:700;color:var(--cream)}

/* Footer CTA */
.footer-cta{padding:5rem 2rem;text-align:center;max-width:600px;margin:0 auto}
.footer-cta h2{font-size:1.8rem;margin-bottom:1rem}
.footer-cta p{color:var(--cream-dim);margin-bottom:2rem;font-style:italic}

/* Footer */
footer{padding:2rem;text-align:center;font-family:var(--font-mono);font-size:0.75rem;color:var(--leather);border-top:1px solid var(--bg3)}
footer .sig{color:var(--leather-light);font-style:italic;font-family:var(--font-serif)}

@media(max-width:600px){
  .hero{padding:4rem 1.5rem 3rem}
  .features-grid{grid-template-columns:1fr}
  .pricing-grid{flex-direction:column;align-items:center}
  .cta-group{flex-direction:column;align-items:center}
}
</style>
</head>
<body>

<nav class="nav">
  <a href="/" class="nav-brand">Stockyard</a>
  <div class="nav-links">
    <a href="/products/">Products</a>
    <a href="/docs/">Docs</a>
    <a href="/pricing/">Pricing</a>
    <a href="https://github.com/stockyard-dev/stockyard">GitHub</a>
  </div>
</nav>

<section class="hero">
  <span class="hero-mark">${key.toUpperCase()}</span>
  <div class="hero-category">${category}</div>
  <h1>${displayName}. <span class="accent">${tagline}.</span></h1>
  <p class="hero-tagline">${solution.split('.')[0]}.</p>
  <div class="cta-group">
    <a href="#pricing" class="btn btn-primary">${cta}</a>
    <a href="/docs/${key}/" class="btn btn-secondary">Read the docs</a>
  </div>
  <code class="install-cmd">npx @stockyard/mcp-${key}</code>
</section>

<section class="pain">
  <div class="pain-inner">
    <div class="pain-label">The problem</div>
    <p class="pain-text">${painPoint}</p>
  </div>
</section>

<section class="solution">
  <div class="solution-label">The fix</div>
  <p class="solution-text">${solution}</p>
</section>

<section class="features">
  <div class="features-inner">
    <div class="features-label">What you get</div>
    <div class="features-grid">
      ${features.map(f => `<div class="feature-item"><span>${f}</span></div>`).join('\n      ')}
    </div>
  </div>
</section>

<section class="quickstart">
  <div class="quickstart-label">30-second quickstart</div>
  <div class="code-block">
<span class="comment"># Install and run</span>
<span class="cmd">npx @stockyard/mcp-${key}</span>

<span class="comment"># Or with Docker</span>
<span class="cmd">docker run -e OPENAI_API_KEY stockyard/${key}</span>

<span class="comment"># Point your app at the proxy</span>
<span class="cmd">export OPENAI_BASE_URL=http://localhost:${key === 'stockyard' ? '4000' : (PRODUCTS.find(p => p[0] === key) || ['','',' ','','','','','','']).toString().split(',')[0]}/v1</span>

<span class="comment"># That's it. Every LLM call now goes through ${displayName}.</span>
  </div>
</section>

<section class="pricing" id="pricing">
  <div class="pricing-label">Pricing</div>
  ${pricingNote}
</section>

<section class="footer-cta">
  <h2>${cta}.</h2>
  <p>Single binary. No dependencies. Works in 30 seconds.</p>
  <div class="cta-group">
    <a href="#pricing" class="btn btn-primary">Start free</a>
    <a href="/products/stockyard/" class="btn btn-secondary">See all 125 products →</a>
  </div>
</section>

<footer>
  <p class="sig">Stockyard.</p>
  <p style="margin-top:0.5rem">Where LLM traffic gets sorted. &nbsp;•&nbsp; 125 products &nbsp;•&nbsp; One binary.</p>
</footer>

</body>
</html>`;
}

// ─── Generate all pages ───────────────────────────────────────────────
const siteDir = path.join(__dirname, "site", "products");
let count = 0;

for (const product of PRODUCTS) {
  const [key, displayName, tagline, painPoint, solution, icon, category, features, cta] = product;
  const dir = path.join(siteDir, key);
  fs.mkdirSync(dir, { recursive: true });
  const html = generatePage(key, displayName, tagline, painPoint, solution, icon, category, features, cta);
  fs.writeFileSync(path.join(dir, "index.html"), html);
  count++;
}

console.log(`\n✅ Generated ${count} landing pages`);
console.log(`   Location: site/products/{key}/index.html`);
console.log(`   Total HTML files: ${count}`);

// Generate products index page
const indexHtml = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>All 125 Products | Stockyard — Where LLM Traffic Gets Sorted</title>
<meta name="description" content="125 LLM infrastructure tools. Cost control, caching, routing, safety, compliance, analytics, and more. Single Go binary.">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;--rust:#c45d2c;--rust-light:#e8753a;--leather:#a0845c;--leather-light:#c4a87a;--cream:#f0e6d3;--cream-dim:#bfb5a3;--gold:#d4a843;--font-serif:'Libre Baskerville',Georgia,serif;--font-mono:'JetBrains Mono',monospace}
body{background:var(--bg);color:var(--cream);font-family:var(--font-serif);line-height:1.7}
a{color:var(--rust-light);text-decoration:none}a:hover{color:var(--gold)}
.nav{padding:1rem 2rem;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--bg3)}
.nav-brand{font-family:var(--font-mono);font-size:0.9rem;color:var(--leather-light);letter-spacing:2px;text-transform:uppercase}
.nav-links{display:flex;gap:1.5rem;font-size:0.85rem;font-family:var(--font-mono)}.nav-links a{color:var(--cream-dim)}
.hero{max-width:900px;margin:0 auto;padding:4rem 2rem 2rem;text-align:center}
.hero h1{font-size:2.5rem;margin-bottom:1rem}.hero h1 .num{color:var(--rust-light)}
.hero p{color:var(--cream-dim);font-style:italic;font-size:1.1rem;margin-bottom:2rem}
.search{max-width:500px;margin:0 auto 3rem}
.search input{width:100%;padding:0.8rem 1.2rem;background:var(--bg2);border:1px solid var(--bg3);color:var(--cream);font-family:var(--font-mono);font-size:0.85rem;outline:none}
.search input:focus{border-color:var(--rust)}
.search input::placeholder{color:var(--leather)}
.grid{max-width:1100px;margin:0 auto;padding:0 2rem 4rem;display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:1rem}
.card{background:var(--bg2);border:1px solid var(--bg3);padding:1.2rem;transition:border-color 0.2s;display:block;color:var(--cream)}
.card:hover{border-color:var(--rust);color:var(--cream)}
.card-top{display:flex;align-items:center;gap:0.8rem;margin-bottom:0.5rem}
.card-name{font-family:var(--font-mono);font-size:0.85rem;color:var(--cream)}
.card-cat{font-family:var(--font-mono);font-size:0.6rem;text-transform:uppercase;letter-spacing:2px;color:var(--rust);margin-left:auto}
.card-tagline{font-size:0.85rem;color:var(--cream-dim)}
footer{padding:2rem;text-align:center;font-family:var(--font-mono);font-size:0.75rem;color:var(--leather);border-top:1px solid var(--bg3)}
</style>
</head>
<body>
<nav class="nav">
  <a href="/" class="nav-brand">Stockyard</a>
  <div class="nav-links"><a href="/products/">Products</a><a href="/docs/">Docs</a><a href="/pricing/">Pricing</a><a href="https://github.com/stockyard-dev/stockyard">GitHub</a></div>
</nav>
<section class="hero">
  <h1><span class="num">125</span> products. One binary.</h1>
  <p>Every tool you need to run LLM traffic in production.</p>
</section>
<div class="search"><input type="text" id="search" placeholder="Search products..." oninput="filterCards(this.value)"></div>
<div class="grid" id="grid">
${PRODUCTS.map(([key, name, tagline, , , , cat]) => 
  `  <a href="/products/${key}/" class="card" data-search="${name.toLowerCase()} ${tagline.toLowerCase()} ${cat.toLowerCase()} ${key}">
    <div class="card-top"><span class="card-name">${name}</span><span class="card-cat">${cat}</span></div>
    <div class="card-tagline">${tagline}</div>
  </a>`).join('\n')}
</div>
<footer><p style="font-style:italic;font-family:var(--font-serif);color:var(--leather-light)">Stockyard.</p><p style="margin-top:0.5rem">Where LLM traffic gets sorted.</p></footer>
<script>
function filterCards(q) {
  q = q.toLowerCase();
  document.querySelectorAll('.card').forEach(c => {
    c.style.display = c.dataset.search.includes(q) ? '' : 'none';
  });
}
</script>
</body>
</html>`;

fs.mkdirSync(siteDir, { recursive: true });
fs.writeFileSync(path.join(siteDir, "index.html"), indexHtml);
console.log(`   + Products index page`);
