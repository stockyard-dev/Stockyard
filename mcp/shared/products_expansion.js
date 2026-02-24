/**
 * Stockyard Expansion Product Definitions (Phase 3 P2/P3 + Phase 4)
 * 93 additional products. Merged into PRODUCTS at runtime.
 */

const EXPANSION_PRODUCTS = {
  agentguard: {
    binary: "agentguard",
    port: 5690,
    displayName: "AgentGuard",
    tagline: "Safety rails for autonomous AI agents",
    description: "Per-session limits for AI agents: max calls, cost, duration. Kill runaway agent sessions before they drain your budget.",
    keywords: ["mcp","mcp-server","llm","agent","safety","session","limits","autonomous","cost-control","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5690, data_dir: "~/.stockyard", log_level: "info", product: "agentguard",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "agentguard_sessions", description: "List active agent sessions with call counts, cost, and duration.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "agentguard_kill", description: "Kill a specific agent session by ID.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "agentguard_stats", description: "Get aggregate stats: sessions tracked, killed, costs saved.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "agentguard_proxy_status", description: "Check if the AgentGuard proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  codefence: {
    binary: "codefence",
    port: 5700,
    displayName: "CodeFence",
    tagline: "Validate LLM-generated code before it runs",
    description: "Scan LLM code output for dangerous patterns: shell injection, file access, crypto mining. Block or flag unsafe code.",
    keywords: ["mcp","mcp-server","llm","code","security","validation","sandbox","patterns","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5700, data_dir: "~/.stockyard", log_level: "info", product: "codefence",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "codefence_stats", description: "Get code validation stats: scanned, flagged, blocked.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "codefence_patterns", description: "List active forbidden patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "codefence_add_pattern", description: "Add a custom forbidden code pattern.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats", method: "POST" },
      { name: "codefence_proxy_status", description: "Check if the CodeFence proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  hallucicheck: {
    binary: "hallucicheck",
    port: 5710,
    displayName: "HalluciCheck",
    tagline: "Catch LLM hallucinations before your users do",
    description: "Validate URLs, emails, and citations in LLM responses. Flag or retry when models invent non-existent references.",
    keywords: ["mcp","mcp-server","llm","hallucination","validation","urls","fact-check","quality","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5710, data_dir: "~/.stockyard", log_level: "info", product: "hallucicheck",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "hallucicheck_stats", description: "Get hallucination detection stats: checked, invalid URLs/emails found.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "hallucicheck_recent", description: "List recent hallucination detections with details.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "hallucicheck_proxy_status", description: "Check if the HalluciCheck proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  tierdrop: {
    binary: "tierdrop",
    port: 5720,
    displayName: "TierDrop",
    tagline: "Auto-downgrade models when burning cash",
    description: "Gracefully degrade from GPT-4 to GPT-3.5 when approaching budget limits. Cost-aware model selection.",
    keywords: ["mcp","mcp-server","llm","cost","downgrade","model","budget","tier","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5720, data_dir: "~/.stockyard", log_level: "info", product: "tierdrop",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "tierdrop_stats", description: "Get downgrade stats: triggers, models switched, savings.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "tierdrop_tiers", description: "List configured cost tiers and thresholds.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "tierdrop_proxy_status", description: "Check if the TierDrop proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  driftwatch: {
    binary: "driftwatch",
    port: 5730,
    displayName: "DriftWatch",
    tagline: "Detect when model behavior changes",
    description: "Track latency and output patterns per model over time. Alert when behavior drifts beyond thresholds.",
    keywords: ["mcp","mcp-server","llm","drift","monitoring","quality","baseline","regression","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5730, data_dir: "~/.stockyard", log_level: "info", product: "driftwatch",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "driftwatch_stats", description: "Get drift detection stats per model.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "driftwatch_baselines", description: "View current baselines for tracked models.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "driftwatch_proxy_status", description: "Check if the DriftWatch proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  feedbackloop: {
    binary: "feedbackloop",
    port: 5740,
    displayName: "FeedbackLoop",
    tagline: "Close the LLM improvement loop",
    description: "Collect user ratings and feedback linked to specific LLM requests. Track quality trends over time.",
    keywords: ["mcp","mcp-server","llm","feedback","ratings","quality","improvement","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5740, data_dir: "~/.stockyard", log_level: "info", product: "feedbackloop",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "feedbackloop_stats", description: "Get feedback stats: total ratings, average score, trends.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "feedbackloop_submit", description: "Submit feedback for a request.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats", method: "POST" },
      { name: "feedbackloop_recent", description: "List recent feedback entries.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "feedbackloop_proxy_status", description: "Check if the FeedbackLoop proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  abrouter: {
    binary: "abrouter",
    port: 5750,
    displayName: "ABRouter",
    tagline: "A/B test any LLM variable with statistical rigor",
    description: "Run experiments across models, prompts, temperatures. Weighted traffic splits with automatic significance testing.",
    keywords: ["mcp","mcp-server","llm","ab-test","experiment","statistics","split","optimization","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5750, data_dir: "~/.stockyard", log_level: "info", product: "abrouter",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "abrouter_experiments", description: "List active experiments with variant stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "abrouter_create", description: "Create a new A/B experiment.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats", method: "POST" },
      { name: "abrouter_results", description: "Get statistical results for an experiment.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "abrouter_proxy_status", description: "Check if the ABRouter proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  guardrail: {
    binary: "guardrail",
    port: 5760,
    displayName: "GuardRail",
    tagline: "Keep your LLM on-script",
    description: "Topic fencing middleware. Define allowed/denied topics. Block off-topic responses with custom fallback messages.",
    keywords: ["mcp","mcp-server","llm","guardrail","topic","filter","boundary","safety","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5760, data_dir: "~/.stockyard", log_level: "info", product: "guardrail",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "guardrail_stats", description: "Get topic enforcement stats: blocked, allowed, violations.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "guardrail_topics", description: "List allowed and denied topic patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "guardrail_proxy_status", description: "Check if the GuardRail proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  geminishim: {
    binary: "geminishim",
    port: 5770,
    displayName: "GeminiShim",
    tagline: "Tame Gemini's quirks behind clean API",
    description: "Handle Gemini safety filter blocks with auto-retry. Normalize token counts. OpenAI-compatible surface for Gemini.",
    keywords: ["mcp","mcp-server","llm","gemini","google","compatibility","shim","safety-filter","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5770, data_dir: "~/.stockyard", log_level: "info", product: "geminishim",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "geminishim_stats", description: "Get Gemini compatibility stats: retries, safety blocks, normalizations.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "geminishim_proxy_status", description: "Check if the GeminiShim proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  localsync: {
    binary: "localsync",
    port: 5780,
    displayName: "LocalSync",
    tagline: "Seamlessly blend local and cloud models",
    description: "Route to Ollama locally when available. Auto-failover to cloud when local is down. Track cost savings.",
    keywords: ["mcp","mcp-server","llm","local","ollama","hybrid","failover","cost","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5780, data_dir: "~/.stockyard", log_level: "info", product: "localsync",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "localsync_stats", description: "Get routing stats: local vs cloud, savings, failovers.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "localsync_health", description: "Check local endpoint health.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "localsync_proxy_status", description: "Check if the LocalSync proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  devproxy: {
    binary: "devproxy",
    port: 5790,
    displayName: "DevProxy",
    tagline: "Charles Proxy for LLM APIs",
    description: "Interactive debugging proxy. Log headers, bodies, latency for every request. Development inspection tool.",
    keywords: ["mcp","mcp-server","llm","debug","inspect","development","logging","headers","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5790, data_dir: "~/.stockyard", log_level: "info", product: "devproxy",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "devproxy_stats", description: "Get debug stats: requests logged, avg latency.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "devproxy_recent", description: "List recent requests with headers and timing.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "devproxy_proxy_status", description: "Check if the DevProxy proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptslim: {
    binary: "promptslim",
    port: 5800,
    displayName: "PromptSlim",
    tagline: "Compress prompts by 40-70% without losing meaning",
    description: "Remove redundant whitespace, filler words, articles. Configurable aggressiveness. See before/after token savings.",
    keywords: ["mcp","mcp-server","llm","prompt","compression","tokens","cost","optimization","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5800, data_dir: "~/.stockyard", log_level: "info", product: "promptslim",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "promptslim_stats", description: "Get compression stats: chars saved, tokens saved, compression ratio.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptslim_proxy_status", description: "Check if the PromptSlim proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptlint: {
    binary: "promptlint",
    port: 5810,
    displayName: "PromptLint",
    tagline: "Catch prompt anti-patterns before they cost you money",
    description: "Static analysis for prompts: detect redundancy, injection patterns, excessive length. Score and suggest improvements.",
    keywords: ["mcp","mcp-server","llm","prompt","lint","analysis","quality","patterns","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5810, data_dir: "~/.stockyard", log_level: "info", product: "promptlint",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "promptlint_stats", description: "Get lint stats: issues found by severity, top patterns.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptlint_proxy_status", description: "Check if the PromptLint proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  approvalgate: {
    binary: "approvalgate",
    port: 5820,
    displayName: "ApprovalGate",
    tagline: "Require human approval for prompt changes",
    description: "Approval workflow for prompt modifications. Track who approved what and when. Audit trail included.",
    keywords: ["mcp","mcp-server","llm","approval","workflow","governance","audit","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5820, data_dir: "~/.stockyard", log_level: "info", product: "approvalgate",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "approvalgate_stats", description: "Get approval stats: pending, approved, rejected.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "approvalgate_pending", description: "List pending approval requests.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "approvalgate_proxy_status", description: "Check if the ApprovalGate proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  outputcap: {
    binary: "outputcap",
    port: 5830,
    displayName: "OutputCap",
    tagline: "Stop paying for responses you don't need",
    description: "Cap output length at natural sentence boundaries. No more 500-token essays when you asked for one word.",
    keywords: ["mcp","mcp-server","llm","output","length","cap","cost","truncation","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5830, data_dir: "~/.stockyard", log_level: "info", product: "outputcap",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "outputcap_stats", description: "Get capping stats: tokens saved, avg reduction.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "outputcap_proxy_status", description: "Check if the OutputCap proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  agegate: {
    binary: "agegate",
    port: 5840,
    displayName: "AgeGate",
    tagline: "Child safety middleware for LLM apps",
    description: "Age-appropriate content filtering. Tiers: child, teen, adult. Injects safety prompts, filters output. COPPA/KOSA ready.",
    keywords: ["mcp","mcp-server","llm","child-safety","age","coppa","filter","content","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5840, data_dir: "~/.stockyard", log_level: "info", product: "agegate",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "agegate_stats", description: "Get safety stats: content filtered, tier distribution.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "agegate_proxy_status", description: "Check if the AgeGate proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  voicebridge: {
    binary: "voicebridge",
    port: 5850,
    displayName: "VoiceBridge",
    tagline: "LLM middleware for voice/TTS pipelines",
    description: "Strip markdown, URLs, code blocks from responses. Convert to speakable prose for voice assistants.",
    keywords: ["mcp","mcp-server","llm","voice","tts","speech","markdown","cleanup","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5850, data_dir: "~/.stockyard", log_level: "info", product: "voicebridge",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "voicebridge_stats", description: "Get voice optimization stats: elements stripped, avg length.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "voicebridge_proxy_status", description: "Check if the VoiceBridge proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  imageproxy: {
    binary: "imageproxy",
    port: 5860,
    displayName: "ImageProxy",
    tagline: "Proxy magic for image generation APIs",
    description: "Cost tracking, caching, and failover for DALL-E and other image generation APIs.",
    keywords: ["mcp","mcp-server","llm","image","dalle","generation","cache","cost","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5860, data_dir: "~/.stockyard", log_level: "info", product: "imageproxy",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "imageproxy_stats", description: "Get image proxy stats: requests, cache hits, cost.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "imageproxy_proxy_status", description: "Check if the ImageProxy proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  langbridge: {
    binary: "langbridge",
    port: 5870,
    displayName: "LangBridge",
    tagline: "Cross-language translation for multilingual apps",
    description: "Auto-detect language, translate to English for model, translate response back. Seamless multilingual support.",
    keywords: ["mcp","mcp-server","llm","translation","multilingual","language","i18n","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5870, data_dir: "~/.stockyard", log_level: "info", product: "langbridge",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "langbridge_stats", description: "Get translation stats: languages detected, translations performed.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "langbridge_proxy_status", description: "Check if the LangBridge proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  contextwindow: {
    binary: "contextwindow",
    port: 5880,
    displayName: "ContextWindow",
    tagline: "Visual context window debugger",
    description: "Visualize token allocation by message role. See what's eating your context window. Optimization recommendations.",
    keywords: ["mcp","mcp-server","llm","context","tokens","debug","visualization","optimization","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5880, data_dir: "~/.stockyard", log_level: "info", product: "contextwindow",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "contextwindow_stats", description: "Get context window analysis: breakdown by role, total usage.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "contextwindow_proxy_status", description: "Check if the ContextWindow proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  regionroute: {
    binary: "regionroute",
    port: 5890,
    displayName: "RegionRoute",
    tagline: "Data residency routing for GDPR compliance",
    description: "Route requests to region-specific endpoints. Keep EU data in EU. Geographic compliance made easy.",
    keywords: ["mcp","mcp-server","llm","gdpr","region","routing","compliance","data-residency","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5890, data_dir: "~/.stockyard", log_level: "info", product: "regionroute",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "regionroute_stats", description: "Get routing stats: requests per region.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "regionroute_routes", description: "List configured region routes.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "regionroute_proxy_status", description: "Check if the RegionRoute proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  chainforge: {
    binary: "chainforge",
    port: 5900,
    displayName: "ChainForge",
    tagline: "Multi-step LLM workflows as YAML pipelines",
    description: "Define extract→analyze→summarize→format pipelines. Conditional branching, parallel execution, cost tracking per pipeline.",
    keywords: ["mcp","mcp-server","llm","pipeline","workflow","chain","multi-step","orchestration","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5900, data_dir: "~/.stockyard", log_level: "info", product: "chainforge",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "chainforge_stats", description: "Get pipeline execution stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "chainforge_pipelines", description: "List configured pipelines.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "chainforge_proxy_status", description: "Check if the ChainForge proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  cronllm: {
    binary: "cronllm",
    port: 5910,
    displayName: "CronLLM",
    tagline: "Scheduled LLM tasks — your AI cron job runner",
    description: "Define scheduled prompts in YAML. Daily summaries, weekly reports, periodic checks. Runs through full proxy chain.",
    keywords: ["mcp","mcp-server","llm","cron","schedule","automation","tasks","periodic","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5910, data_dir: "~/.stockyard", log_level: "info", product: "cronllm",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "cronllm_stats", description: "Get job execution stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "cronllm_jobs", description: "List scheduled jobs.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "cronllm_proxy_status", description: "Check if the CronLLM proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  webhookrelay: {
    binary: "webhookrelay",
    port: 5920,
    displayName: "WebhookRelay",
    tagline: "Trigger LLM calls from any webhook",
    description: "Receive webhooks, extract data, build prompts, call LLM, send results. GitHub→summarize→Slack in one config.",
    keywords: ["mcp","mcp-server","llm","webhook","trigger","event-driven","automation","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5920, data_dir: "~/.stockyard", log_level: "info", product: "webhookrelay",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "webhookrelay_stats", description: "Get relay stats: webhooks received, calls triggered.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "webhookrelay_triggers", description: "List configured webhook triggers.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "webhookrelay_proxy_status", description: "Check if the WebhookRelay proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  billsync: {
    binary: "billsync",
    port: 5930,
    displayName: "BillSync",
    tagline: "Per-customer LLM invoices automatically",
    description: "Track usage per tenant. Apply markup. Generate invoice data. Stripe-compatible usage records.",
    keywords: ["mcp","mcp-server","llm","billing","invoice","tenant","markup","saas","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5930, data_dir: "~/.stockyard", log_level: "info", product: "billsync",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "billsync_stats", description: "Get billing stats: tenants, revenue, markup.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "billsync_tenants", description: "List tenant billing summaries.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "billsync_proxy_status", description: "Check if the BillSync proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  whitelabel: {
    binary: "whitelabel",
    port: 5940,
    displayName: "WhiteLabel",
    tagline: "Your brand on Stockyard's engine",
    description: "Custom branding for resellers. Logo, colors, domain. Sell LLM infrastructure under your own brand.",
    keywords: ["mcp","mcp-server","llm","branding","white-label","reseller","custom","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5940, data_dir: "~/.stockyard", log_level: "info", product: "whitelabel",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "whitelabel_stats", description: "Get branding stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "whitelabel_proxy_status", description: "Check if the WhiteLabel proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  trainexport: {
    binary: "trainexport",
    port: 5950,
    displayName: "TrainExport",
    tagline: "Export LLM conversations as fine-tuning datasets",
    description: "Collect input/output pairs from live traffic. Export as OpenAI JSONL, Anthropic, or Alpaca format.",
    keywords: ["mcp","mcp-server","llm","training","fine-tuning","export","dataset","jsonl","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5950, data_dir: "~/.stockyard", log_level: "info", product: "trainexport",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "trainexport_stats", description: "Get collection stats: pairs collected, storage used.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "trainexport_export", description: "Export collected pairs in specified format.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats", method: "POST" },
      { name: "trainexport_proxy_status", description: "Check if the TrainExport proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  synthgen: {
    binary: "synthgen",
    port: 5960,
    displayName: "SynthGen",
    tagline: "Generate synthetic training data through your proxy",
    description: "Templates + seed examples → synthetic training data at scale. Quality-checked through EvalGate.",
    keywords: ["mcp","mcp-server","llm","synthetic","training","generation","data","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5960, data_dir: "~/.stockyard", log_level: "info", product: "synthgen",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "synthgen_stats", description: "Get generation stats: samples generated, batches run.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "synthgen_proxy_status", description: "Check if the SynthGen proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  diffprompt: {
    binary: "diffprompt",
    port: 5970,
    displayName: "DiffPrompt",
    tagline: "Git-style diff for prompt changes",
    description: "Track system prompt changes. Hash-based detection. See which models had prompt modifications.",
    keywords: ["mcp","mcp-server","llm","diff","prompt","versioning","change-detection","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5970, data_dir: "~/.stockyard", log_level: "info", product: "diffprompt",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "diffprompt_stats", description: "Get change detection stats: prompts checked, changes detected.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "diffprompt_proxy_status", description: "Check if the DiffPrompt proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  llmbench: {
    binary: "llmbench",
    port: 5980,
    displayName: "LLMBench",
    tagline: "Benchmark any model on YOUR workload",
    description: "Per-model performance tracking: latency, cost, tokens. Compare models on your actual traffic.",
    keywords: ["mcp","mcp-server","llm","benchmark","performance","comparison","latency","cost","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5980, data_dir: "~/.stockyard", log_level: "info", product: "llmbench",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "llmbench_stats", description: "Get benchmark results per model.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "llmbench_compare", description: "Compare two models side by side.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "llmbench_proxy_status", description: "Check if the LLMBench proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  maskmode: {
    binary: "maskmode",
    port: 5990,
    displayName: "MaskMode",
    tagline: "Demo mode with realistic fake data",
    description: "Replace real PII in responses with realistic fakes. Consistent within session. Perfect for sales demos.",
    keywords: ["mcp","mcp-server","llm","demo","mask","pii","fake-data","sales","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 5990, data_dir: "~/.stockyard", log_level: "info", product: "maskmode",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "maskmode_stats", description: "Get masking stats: requests masked, replacements made.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "maskmode_proxy_status", description: "Check if the MaskMode proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  tokenmarket: {
    binary: "tokenmarket",
    port: 6000,
    displayName: "TokenMarket",
    tagline: "Dynamic budget reallocation across teams",
    description: "Pool-based budgets. Teams request capacity. Auto-rebalance. Priority queuing for high-value requests.",
    keywords: ["mcp","mcp-server","llm","budget","pool","reallocation","teams","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6000, data_dir: "~/.stockyard", log_level: "info", product: "tokenmarket",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "tokenmarket_stats", description: "Get market stats: pool balances, transactions.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "tokenmarket_pools", description: "List budget pools with current balances.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "tokenmarket_proxy_status", description: "Check if the TokenMarket proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  llmsync: {
    binary: "llmsync",
    port: 6010,
    displayName: "LLMSync",
    tagline: "Replicate config across environments",
    description: "Environment hierarchy with config inheritance. Diff, promote, rollback. Git-friendly YAML management.",
    keywords: ["mcp","mcp-server","llm","sync","config","environment","deployment","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6010, data_dir: "~/.stockyard", log_level: "info", product: "llmsync",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "llmsync_stats", description: "Get sync stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "llmsync_proxy_status", description: "Check if the LLMSync proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  clustermode: {
    binary: "clustermode",
    port: 6020,
    displayName: "ClusterMode",
    tagline: "Run multiple instances with shared state",
    description: "Multi-instance coordination. Leader-follower with shared cache. Scale beyond single-instance SQLite.",
    keywords: ["mcp","mcp-server","llm","cluster","scale","multi-instance","coordination","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6020, data_dir: "~/.stockyard", log_level: "info", product: "clustermode",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "clustermode_stats", description: "Get cluster stats: nodes, requests distributed.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "clustermode_nodes", description: "List cluster nodes and their status.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "clustermode_proxy_status", description: "Check if the ClusterMode proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  encryptvault: {
    binary: "encryptvault",
    port: 6030,
    displayName: "EncryptVault",
    tagline: "End-to-end encryption for sensitive LLM payloads",
    description: "AES-GCM encryption for sensitive fields. Customer-managed keys. HIPAA/SOC2 compliance ready.",
    keywords: ["mcp","mcp-server","llm","encryption","security","hipaa","compliance","vault","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6030, data_dir: "~/.stockyard", log_level: "info", product: "encryptvault",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "encryptvault_stats", description: "Get encryption stats: fields encrypted/decrypted.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "encryptvault_proxy_status", description: "Check if the EncryptVault proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  mirrortest: {
    binary: "mirrortest",
    port: 6040,
    displayName: "MirrorTest",
    tagline: "Shadow test new models against production traffic",
    description: "Send production traffic to a shadow model. Compare quality, latency, cost. Zero user impact.",
    keywords: ["mcp","mcp-server","llm","shadow","testing","comparison","canary","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6040, data_dir: "~/.stockyard", log_level: "info", product: "mirrortest",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "mirrortest_stats", description: "Get shadow test stats: requests mirrored, success rates.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "mirrortest_proxy_status", description: "Check if the MirrorTest proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  extractml: {
    binary: "extractml",
    port: 6050,
    displayName: "ExtractML",
    tagline: "Turn unstructured LLM responses into structured data",
    description: "Force extraction from free-text into JSON when models return prose.",
    keywords: ["mcp","mcp-server","llm","extraction","structured","json","parsing","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6050, data_dir: "~/.stockyard", log_level: "info", product: "extractml",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "extractml_stats", description: "Get extraction stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "extractml_proxy_status", description: "Check if the ExtractML proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  tableforge: {
    binary: "tableforge",
    port: 6060,
    displayName: "TableForge",
    tagline: "LLM-powered CSV/table generation with validation",
    description: "Detect tables in output. Validate columns, types, completeness. Auto-repair and export.",
    keywords: ["mcp","mcp-server","llm","table","csv","validation","structured","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6060, data_dir: "~/.stockyard", log_level: "info", product: "tableforge",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "tableforge_stats", description: "Get table validation stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "tableforge_proxy_status", description: "Check if the TableForge proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  toolrouter: {
    binary: "toolrouter",
    port: 6070,
    displayName: "ToolRouter",
    tagline: "Manage, version, and route LLM function calls",
    description: "Versioned tool schemas. Route calls. Shadow-test. Usage analytics.",
    keywords: ["mcp","mcp-server","llm","tools","function-calling","routing","versioning","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6070, data_dir: "~/.stockyard", log_level: "info", product: "toolrouter",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "toolrouter_stats", description: "Get tool routing stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "toolrouter_tools", description: "List registered tools.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "toolrouter_proxy_status", description: "Check if the ToolRouter proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  toolshield: {
    binary: "toolshield",
    port: 6080,
    displayName: "ToolShield",
    tagline: "Validate and sandbox LLM tool calls",
    description: "Intercept tool_use. Validate args. Per-tool permissions and rate limits.",
    keywords: ["mcp","mcp-server","llm","tools","validation","sandbox","permissions","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6080, data_dir: "~/.stockyard", log_level: "info", product: "toolshield",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "toolshield_stats", description: "Get tool validation stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "toolshield_proxy_status", description: "Check if the ToolShield proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  toolmock: {
    binary: "toolmock",
    port: 6090,
    displayName: "ToolMock",
    tagline: "Fake tool responses for testing",
    description: "Canned responses by tool+args. Simulate errors, timeouts, partial results.",
    keywords: ["mcp","mcp-server","llm","tools","mock","testing","simulation","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6090, data_dir: "~/.stockyard", log_level: "info", product: "toolmock",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "toolmock_stats", description: "Get mock stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "toolmock_proxy_status", description: "Check if the ToolMock proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  authgate: {
    binary: "authgate",
    port: 6100,
    displayName: "AuthGate",
    tagline: "API key management for YOUR users",
    description: "Issue/revoke keys to your customers. Per-key limits and usage tracking.",
    keywords: ["mcp","mcp-server","llm","auth","api-keys","management","customers","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6100, data_dir: "~/.stockyard", log_level: "info", product: "authgate",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "authgate_stats", description: "Get auth stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "authgate_keys", description: "List API keys.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "authgate_proxy_status", description: "Check if the AuthGate proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  scopeguard: {
    binary: "scopeguard",
    port: 6110,
    displayName: "ScopeGuard",
    tagline: "Fine-grained permissions per API key",
    description: "Role-based access control. Map keys to allowed models, endpoints, features.",
    keywords: ["mcp","mcp-server","llm","permissions","rbac","scope","access-control","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6110, data_dir: "~/.stockyard", log_level: "info", product: "scopeguard",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "scopeguard_stats", description: "Get permission stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "scopeguard_roles", description: "List roles.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "scopeguard_proxy_status", description: "Check if the ScopeGuard proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  visionproxy: {
    binary: "visionproxy",
    port: 6120,
    displayName: "VisionProxy",
    tagline: "Proxy magic for vision/image APIs",
    description: "Caching, cost tracking, and failover for GPT-4V, Claude vision.",
    keywords: ["mcp","mcp-server","llm","vision","image","multimodal","cache","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6120, data_dir: "~/.stockyard", log_level: "info", product: "visionproxy",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "visionproxy_stats", description: "Get vision proxy stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "visionproxy_proxy_status", description: "Check if the VisionProxy proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  audioproxy: {
    binary: "audioproxy",
    port: 6130,
    displayName: "AudioProxy",
    tagline: "Proxy for speech-to-text and text-to-speech",
    description: "Cache TTS, track per-minute costs, failover between STT/TTS providers.",
    keywords: ["mcp","mcp-server","llm","audio","stt","tts","speech","whisper","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6130, data_dir: "~/.stockyard", log_level: "info", product: "audioproxy",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "audioproxy_stats", description: "Get audio proxy stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "audioproxy_proxy_status", description: "Check if the AudioProxy proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  docparse: {
    binary: "docparse",
    port: 6140,
    displayName: "DocParse",
    tagline: "Preprocess documents before they hit the LLM",
    description: "PDF/Word/HTML text extraction. Smart chunking. Clean artifacts.",
    keywords: ["mcp","mcp-server","llm","document","parsing","chunking","pdf","rag","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6140, data_dir: "~/.stockyard", log_level: "info", product: "docparse",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "docparse_stats", description: "Get document processing stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "docparse_proxy_status", description: "Check if the DocParse proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  framegrab: {
    binary: "framegrab",
    port: 6150,
    displayName: "FrameGrab",
    tagline: "Extract and analyze video frames through vision LLMs",
    description: "Scene detection. Batch frames. Smart frame selection. Cost per frame.",
    keywords: ["mcp","mcp-server","llm","video","frames","vision","analysis","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6150, data_dir: "~/.stockyard", log_level: "info", product: "framegrab",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "framegrab_stats", description: "Get frame extraction stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "framegrab_proxy_status", description: "Check if the FrameGrab proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  sessionstore: {
    binary: "sessionstore",
    port: 6160,
    displayName: "SessionStore",
    tagline: "Managed conversation sessions",
    description: "Create/resume/list/delete sessions. Full history. Metadata. Concurrent limits.",
    keywords: ["mcp","mcp-server","llm","session","conversation","history","management","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6160, data_dir: "~/.stockyard", log_level: "info", product: "sessionstore",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "sessionstore_stats", description: "Get session stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "sessionstore_sessions", description: "List active sessions.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "sessionstore_proxy_status", description: "Check if the SessionStore proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  convofork: {
    binary: "convofork",
    port: 6170,
    displayName: "ConvoFork",
    tagline: "Branch conversations — try different paths",
    description: "Fork at any message. Independent history per branch. Tree visualization.",
    keywords: ["mcp","mcp-server","llm","fork","branch","conversation","parallel","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6170, data_dir: "~/.stockyard", log_level: "info", product: "convofork",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "convofork_stats", description: "Get fork stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "convofork_proxy_status", description: "Check if the ConvoFork proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  slotfill: {
    binary: "slotfill",
    port: 6180,
    displayName: "SlotFill",
    tagline: "Form-filling conversation engine",
    description: "Declarative slot definitions. Track filled/missing. Reprompt. Completion funnels.",
    keywords: ["mcp","mcp-server","llm","forms","slots","conversation","intake","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6180, data_dir: "~/.stockyard", log_level: "info", product: "slotfill",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "slotfill_stats", description: "Get slot fill stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "slotfill_proxy_status", description: "Check if the SlotFill proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  semanticcache: {
    binary: "semanticcache",
    port: 6190,
    displayName: "SemanticCache",
    tagline: "Cache hits for similar prompts, not just identical",
    description: "Embed prompts. Cosine similarity. Configurable threshold. 10x hit rate.",
    keywords: ["mcp","mcp-server","llm","cache","semantic","similarity","embeddings","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6190, data_dir: "~/.stockyard", log_level: "info", product: "semanticcache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "semanticcache_stats", description: "Get semantic cache stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "semanticcache_proxy_status", description: "Check if the SemanticCache proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  partialcache: {
    binary: "partialcache",
    port: 6200,
    displayName: "PartialCache",
    tagline: "Cache reusable prompt prefixes",
    description: "Detect static system prompt prefix. Use native prefix caching where supported.",
    keywords: ["mcp","mcp-server","llm","cache","prefix","optimization","tokens","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6200, data_dir: "~/.stockyard", log_level: "info", product: "partialcache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "partialcache_stats", description: "Get prefix cache stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "partialcache_proxy_status", description: "Check if the PartialCache proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  streamcache: {
    binary: "streamcache",
    port: 6210,
    displayName: "StreamCache",
    tagline: "Cache streaming responses with realistic timing",
    description: "Store original chunk timing. Replay cached SSE with original pacing.",
    keywords: ["mcp","mcp-server","llm","cache","streaming","sse","replay","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6210, data_dir: "~/.stockyard", log_level: "info", product: "streamcache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "streamcache_stats", description: "Get stream cache stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "streamcache_proxy_status", description: "Check if the StreamCache proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptchain: {
    binary: "promptchain",
    port: 6220,
    displayName: "PromptChain",
    tagline: "Composable prompt blocks",
    description: "Define reusable blocks. Compose: [tone.helpful, format.json, domain.ecommerce]. Auto-update.",
    keywords: ["mcp","mcp-server","llm","prompt","components","composable","reusable","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6220, data_dir: "~/.stockyard", log_level: "info", product: "promptchain",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "promptchain_stats", description: "Get composition stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptchain_blocks", description: "List defined blocks.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptchain_proxy_status", description: "Check if the PromptChain proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptfuzz: {
    binary: "promptfuzz",
    port: 6230,
    displayName: "PromptFuzz",
    tagline: "Fuzz-test your prompts",
    description: "Generate adversarial, multilingual, edge-case inputs. Score with EvalGate. Report failures.",
    keywords: ["mcp","mcp-server","llm","fuzz","testing","adversarial","prompt","security","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6230, data_dir: "~/.stockyard", log_level: "info", product: "promptfuzz",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "promptfuzz_stats", description: "Get fuzz test stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptfuzz_proxy_status", description: "Check if the PromptFuzz proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptmarket: {
    binary: "promptmarket",
    port: 6240,
    displayName: "PromptMarket",
    tagline: "Community prompt library",
    description: "Publish, browse, rate, fork prompts. Track which community prompts you use.",
    keywords: ["mcp","mcp-server","llm","prompt","marketplace","community","sharing","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6240, data_dir: "~/.stockyard", log_level: "info", product: "promptmarket",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "promptmarket_stats", description: "Get marketplace stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptmarket_proxy_status", description: "Check if the PromptMarket proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  costpredict: {
    binary: "costpredict",
    port: 6250,
    displayName: "CostPredict",
    tagline: "Predict request cost BEFORE sending",
    description: "Count input tokens. Estimate output. Calculate cost. X-Estimated-Cost header.",
    keywords: ["mcp","mcp-server","llm","cost","prediction","estimate","budget","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6250, data_dir: "~/.stockyard", log_level: "info", product: "costpredict",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "costpredict_stats", description: "Get prediction stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "costpredict_proxy_status", description: "Check if the CostPredict proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  costmap: {
    binary: "costmap",
    port: 6260,
    displayName: "CostMap",
    tagline: "Multi-dimensional cost attribution",
    description: "Tag requests with dimensions. Drill-down: by feature, user, prompt.",
    keywords: ["mcp","mcp-server","llm","cost","attribution","analytics","drill-down","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6260, data_dir: "~/.stockyard", log_level: "info", product: "costmap",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "costmap_stats", description: "Get cost attribution stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "costmap_proxy_status", description: "Check if the CostMap proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  spotprice: {
    binary: "spotprice",
    port: 6270,
    displayName: "SpotPrice",
    tagline: "Real-time model pricing intelligence",
    description: "Live pricing DB. Route to cheapest model meeting quality threshold.",
    keywords: ["mcp","mcp-server","llm","pricing","cost","routing","optimization","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6270, data_dir: "~/.stockyard", log_level: "info", product: "spotprice",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "spotprice_stats", description: "Get pricing stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "spotprice_proxy_status", description: "Check if the SpotPrice proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  loadforge: {
    binary: "loadforge",
    port: 6280,
    displayName: "LoadForge",
    tagline: "Load test your LLM stack",
    description: "Define load profiles. Measure TTFT, TPS, p50/p95/p99, errors.",
    keywords: ["mcp","mcp-server","llm","load-test","performance","benchmark","stress","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6280, data_dir: "~/.stockyard", log_level: "info", product: "loadforge",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "loadforge_stats", description: "Get load test results.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "loadforge_proxy_status", description: "Check if the LoadForge proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  snapshottest: {
    binary: "snapshottest",
    port: 6290,
    displayName: "SnapshotTest",
    tagline: "Snapshot testing for LLM outputs",
    description: "Record baselines. Semantic diff. Configurable threshold. CI-friendly.",
    keywords: ["mcp","mcp-server","llm","snapshot","testing","regression","ci-cd","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6290, data_dir: "~/.stockyard", log_level: "info", product: "snapshottest",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "snapshottest_stats", description: "Get snapshot test stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "snapshottest_proxy_status", description: "Check if the SnapshotTest proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  chaosllm: {
    binary: "chaosllm",
    port: 6300,
    displayName: "ChaosLLM",
    tagline: "Chaos engineering for your LLM stack",
    description: "Inject realistic failures: 429s, timeouts, malformed JSON, truncated streams.",
    keywords: ["mcp","mcp-server","llm","chaos","testing","resilience","fault-injection","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6300, data_dir: "~/.stockyard", log_level: "info", product: "chaosllm",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "chaosllm_stats", description: "Get chaos injection stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "chaosllm_proxy_status", description: "Check if the ChaosLLM proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  datamap: {
    binary: "datamap",
    port: 6310,
    displayName: "DataMap",
    tagline: "GDPR Article 30 data flow mapping",
    description: "Auto-classify data. Map flows: source→proxy→provider→storage. Generate GDPR records.",
    keywords: ["mcp","mcp-server","llm","gdpr","compliance","data-flow","mapping","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6310, data_dir: "~/.stockyard", log_level: "info", product: "datamap",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "datamap_stats", description: "Get data mapping stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "datamap_proxy_status", description: "Check if the DataMap proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  consentgate: {
    binary: "consentgate",
    port: 6320,
    displayName: "ConsentGate",
    tagline: "User consent management for AI interactions",
    description: "Check consent per user. Block non-consented. Track timestamps. Support withdrawal.",
    keywords: ["mcp","mcp-server","llm","consent","gdpr","eu-ai-act","compliance","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6320, data_dir: "~/.stockyard", log_level: "info", product: "consentgate",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "consentgate_stats", description: "Get consent stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "consentgate_proxy_status", description: "Check if the ConsentGate proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  retentionwipe: {
    binary: "retentionwipe",
    port: 6330,
    displayName: "RetentionWipe",
    tagline: "Automated data retention and deletion",
    description: "Retention periods per data type. Auto-purge. Per-user deletion. Deletion certificates.",
    keywords: ["mcp","mcp-server","llm","retention","deletion","gdpr","compliance","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6330, data_dir: "~/.stockyard", log_level: "info", product: "retentionwipe",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "retentionwipe_stats", description: "Get retention stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "retentionwipe_proxy_status", description: "Check if the RetentionWipe proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  policyengine: {
    binary: "policyengine",
    port: 6340,
    displayName: "PolicyEngine",
    tagline: "Codify AI governance as enforceable rules",
    description: "YAML policy rules compiled to middleware. Audit log. Compliance rate dashboard.",
    keywords: ["mcp","mcp-server","llm","policy","governance","compliance","rules","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6340, data_dir: "~/.stockyard", log_level: "info", product: "policyengine",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "policyengine_stats", description: "Get policy enforcement stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "policyengine_proxy_status", description: "Check if the PolicyEngine proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  streamsplit: {
    binary: "streamsplit",
    port: 6350,
    displayName: "StreamSplit",
    tagline: "Fork streaming responses to multiple destinations",
    description: "Tee SSE chunks to logger, quality checker, webhook. Zero latency for primary.",
    keywords: ["mcp","mcp-server","llm","streaming","fork","multiplex","sse","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6350, data_dir: "~/.stockyard", log_level: "info", product: "streamsplit",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "streamsplit_stats", description: "Get stream split stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "streamsplit_proxy_status", description: "Check if the StreamSplit proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  streamthrottle: {
    binary: "streamthrottle",
    port: 6360,
    displayName: "StreamThrottle",
    tagline: "Control streaming speed for better UX",
    description: "Max tokens/sec. Buffer fast streams. Per endpoint/model/client.",
    keywords: ["mcp","mcp-server","llm","streaming","throttle","speed","ux","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6360, data_dir: "~/.stockyard", log_level: "info", product: "streamthrottle",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "streamthrottle_stats", description: "Get throttle stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "streamthrottle_proxy_status", description: "Check if the StreamThrottle proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  streamtransform: {
    binary: "streamtransform",
    port: 6370,
    displayName: "StreamTransform",
    tagline: "Transform streaming responses mid-stream",
    description: "Pipeline on chunks: strip markdown, redact PII, translate. Minimal latency.",
    keywords: ["mcp","mcp-server","llm","streaming","transform","pipeline","real-time","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6370, data_dir: "~/.stockyard", log_level: "info", product: "streamtransform",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "streamtransform_stats", description: "Get transform stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "streamtransform_proxy_status", description: "Check if the StreamTransform proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  modelalias: {
    binary: "modelalias",
    port: 6380,
    displayName: "ModelAlias",
    tagline: "Abstract away model names with aliases",
    description: "Aliases: fast→gpt-4o-mini, smart→claude-sonnet. Change mapping, all apps update.",
    keywords: ["mcp","mcp-server","llm","alias","model","abstraction","mapping","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6380, data_dir: "~/.stockyard", log_level: "info", product: "modelalias",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "modelalias_stats", description: "Get alias resolution stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "modelalias_list", description: "List active aliases.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "modelalias_proxy_status", description: "Check if the ModelAlias proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  paramnorm: {
    binary: "paramnorm",
    port: 6390,
    displayName: "ParamNorm",
    tagline: "Normalize parameters across providers",
    description: "Calibration profiles per model. Map normalized params to model-specific ranges.",
    keywords: ["mcp","mcp-server","llm","parameters","normalization","calibration","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6390, data_dir: "~/.stockyard", log_level: "info", product: "paramnorm",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "paramnorm_stats", description: "Get normalization stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "paramnorm_proxy_status", description: "Check if the ParamNorm proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  quotasync: {
    binary: "quotasync",
    port: 6400,
    displayName: "QuotaSync",
    tagline: "Track provider rate limits in real-time",
    description: "Parse rate limit headers. Track per model/endpoint. Alert near limits.",
    keywords: ["mcp","mcp-server","llm","quota","rate-limit","tracking","provider","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6400, data_dir: "~/.stockyard", log_level: "info", product: "quotasync",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "quotasync_stats", description: "Get quota tracking stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "quotasync_proxy_status", description: "Check if the QuotaSync proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  errornorm: {
    binary: "errornorm",
    port: 6410,
    displayName: "ErrorNorm",
    tagline: "Normalize error responses across providers",
    description: "Single error schema: code, message, provider, retry_after, is_retryable.",
    keywords: ["mcp","mcp-server","llm","errors","normalization","consistency","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6410, data_dir: "~/.stockyard", log_level: "info", product: "errornorm",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "errornorm_stats", description: "Get error normalization stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "errornorm_proxy_status", description: "Check if the ErrorNorm proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  cohorttrack: {
    binary: "cohorttrack",
    port: 6420,
    displayName: "CohortTrack",
    tagline: "User cohort analytics for LLM products",
    description: "Cohorts by signup, plan, feature. Retention, cost per cohort. BI export.",
    keywords: ["mcp","mcp-server","llm","analytics","cohort","retention","users","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6420, data_dir: "~/.stockyard", log_level: "info", product: "cohorttrack",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "cohorttrack_stats", description: "Get cohort analytics.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "cohorttrack_proxy_status", description: "Check if the CohortTrack proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  promptrank: {
    binary: "promptrank",
    port: 6430,
    displayName: "PromptRank",
    tagline: "Rank prompts by ROI",
    description: "Per template: cost, quality, latency, volume, feedback. ROI leaderboard.",
    keywords: ["mcp","mcp-server","llm","analytics","prompt","roi","ranking","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6430, data_dir: "~/.stockyard", log_level: "info", product: "promptrank",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "promptrank_stats", description: "Get prompt rankings.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "promptrank_proxy_status", description: "Check if the PromptRank proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  anomalyradar: {
    binary: "anomalyradar",
    port: 6440,
    displayName: "AnomalyRadar",
    tagline: "ML-powered anomaly detection",
    description: "Build statistical baselines. Z-score deviation detection. Auto-adjusting thresholds.",
    keywords: ["mcp","mcp-server","llm","anomaly","detection","monitoring","ml","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6440, data_dir: "~/.stockyard", log_level: "info", product: "anomalyradar",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "anomalyradar_stats", description: "Get anomaly detection stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "anomalyradar_proxy_status", description: "Check if the AnomalyRadar proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  envsync: {
    binary: "envsync",
    port: 6450,
    displayName: "EnvSync",
    tagline: "Sync configs + secrets across environments",
    description: "Push/promote/diff. Encrypted secrets. Pre-promotion validation. Rollback.",
    keywords: ["mcp","mcp-server","llm","sync","secrets","environment","deployment","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6450, data_dir: "~/.stockyard", log_level: "info", product: "envsync",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "envsync_stats", description: "Get sync stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "envsync_proxy_status", description: "Check if the EnvSync proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  proxylog: {
    binary: "proxylog",
    port: 6460,
    displayName: "ProxyLog",
    tagline: "Structured logging for every proxy decision",
    description: "Each middleware emits decision log. Per-request trace. X-Proxy-Trace header.",
    keywords: ["mcp","mcp-server","llm","logging","decisions","trace","debug","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6460, data_dir: "~/.stockyard", log_level: "info", product: "proxylog",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "proxylog_stats", description: "Get logging stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "proxylog_proxy_status", description: "Check if the ProxyLog proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  clidash: {
    binary: "clidash",
    port: 6470,
    displayName: "CliDash",
    tagline: "Terminal dashboard — htop for your LLM stack",
    description: "Real-time TUI: req/sec, models, cache, spend, errors. SSH-accessible.",
    keywords: ["mcp","mcp-server","llm","terminal","dashboard","tui","monitoring","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6470, data_dir: "~/.stockyard", log_level: "info", product: "clidash",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "clidash_stats", description: "Get dashboard data.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "clidash_proxy_status", description: "Check if the CliDash proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  embedrouter: {
    binary: "embedrouter",
    port: 6480,
    displayName: "EmbedRouter",
    tagline: "Smart routing for embedding requests",
    description: "Batch over 50ms window. Deduplicate. Route by content type.",
    keywords: ["mcp","mcp-server","llm","embeddings","routing","batch","dedup","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6480, data_dir: "~/.stockyard", log_level: "info", product: "embedrouter",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "embedrouter_stats", description: "Get embedding routing stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "embedrouter_proxy_status", description: "Check if the EmbedRouter proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  finetunetrack: {
    binary: "finetunetrack",
    port: 6490,
    displayName: "FineTuneTrack",
    tagline: "Monitor fine-tuned model performance",
    description: "Eval suite. Run periodically. Track scores. Compare to base model.",
    keywords: ["mcp","mcp-server","llm","fine-tune","monitoring","evaluation","drift","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6490, data_dir: "~/.stockyard", log_level: "info", product: "finetunetrack",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "finetunetrack_stats", description: "Get fine-tune tracking stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "finetunetrack_proxy_status", description: "Check if the FineTuneTrack proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  agentreplay: {
    binary: "agentreplay",
    port: 6500,
    displayName: "AgentReplay",
    tagline: "Record and replay agent sessions step-by-step",
    description: "Step-by-step playback on TraceLink data. What-if mode. Export as test cases.",
    keywords: ["mcp","mcp-server","llm","agent","replay","debug","session","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6500, data_dir: "~/.stockyard", log_level: "info", product: "agentreplay",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "agentreplay_stats", description: "Get replay stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "agentreplay_proxy_status", description: "Check if the AgentReplay proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  summarizegate: {
    binary: "summarizegate",
    port: 6510,
    displayName: "SummarizeGate",
    tagline: "Auto-summarize long contexts to save tokens",
    description: "Score relevance per section. Keep high-relevance verbatim. Summarize low-relevance.",
    keywords: ["mcp","mcp-server","llm","summarize","context","tokens","optimization","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6510, data_dir: "~/.stockyard", log_level: "info", product: "summarizegate",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "summarizegate_stats", description: "Get summarization stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "summarizegate_proxy_status", description: "Check if the SummarizeGate proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  codelang: {
    binary: "codelang",
    port: 6520,
    displayName: "CodeLang",
    tagline: "Language-aware code generation with syntax validation",
    description: "Tree-sitter parsing. Syntax errors, undefined refs, suspicious patterns.",
    keywords: ["mcp","mcp-server","llm","code","syntax","validation","parsing","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6520, data_dir: "~/.stockyard", log_level: "info", product: "codelang",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "codelang_stats", description: "Get code validation stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "codelang_proxy_status", description: "Check if the CodeLang proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  personaswitch: {
    binary: "personaswitch",
    port: 6530,
    displayName: "PersonaSwitch",
    tagline: "Hot-swap AI personalities without code changes",
    description: "Define personas. Route by header/key/segment. Each: prompt, temperature, rules.",
    keywords: ["mcp","mcp-server","llm","persona","personality","routing","customization","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6530, data_dir: "~/.stockyard", log_level: "info", product: "personaswitch",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "personaswitch_stats", description: "Get persona stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "personaswitch_personas", description: "List personas.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "personaswitch_proxy_status", description: "Check if the PersonaSwitch proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  warmpool: {
    binary: "warmpool",
    port: 6540,
    displayName: "WarmPool",
    tagline: "Pre-warm model connections",
    description: "Persistent connections. Health checks. Keep-alive for Ollama.",
    keywords: ["mcp","mcp-server","llm","warmup","connections","latency","performance","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6540, data_dir: "~/.stockyard", log_level: "info", product: "warmpool",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "warmpool_stats", description: "Get connection pool stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "warmpool_proxy_status", description: "Check if the WarmPool proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  edgecache: {
    binary: "edgecache",
    port: 6550,
    displayName: "EdgeCache",
    tagline: "CDN-like caching for LLM responses",
    description: "Distribute cache across instances. Geographic hit rates.",
    keywords: ["mcp","mcp-server","llm","cache","cdn","edge","distributed","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6550, data_dir: "~/.stockyard", log_level: "info", product: "edgecache",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "edgecache_stats", description: "Get edge cache stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "edgecache_proxy_status", description: "Check if the EdgeCache proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  queuepriority: {
    binary: "queuepriority",
    port: 6560,
    displayName: "QueuePriority",
    tagline: "Priority queues — VIP users first",
    description: "Priority levels per key/tenant. Reserved capacity. SLA tracking.",
    keywords: ["mcp","mcp-server","llm","queue","priority","vip","sla","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6560, data_dir: "~/.stockyard", log_level: "info", product: "queuepriority",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "queuepriority_stats", description: "Get queue stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "queuepriority_proxy_status", description: "Check if the QueuePriority proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  geoprice: {
    binary: "geoprice",
    port: 6570,
    displayName: "GeoPrice",
    tagline: "Purchasing power pricing by region",
    description: "PPP-adjusted pricing. Anti-VPN. Revenue by region dashboard.",
    keywords: ["mcp","mcp-server","llm","pricing","geo","ppp","regional","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6570, data_dir: "~/.stockyard", log_level: "info", product: "geoprice",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "geoprice_stats", description: "Get pricing stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "geoprice_proxy_status", description: "Check if the GeoPrice proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  tokenauction: {
    binary: "tokenauction",
    port: 6580,
    displayName: "TokenAuction",
    tagline: "Dynamic pricing based on demand",
    description: "Monitor costs, queue, errors. Time-of-day pricing. Surge pricing.",
    keywords: ["mcp","mcp-server","llm","pricing","dynamic","auction","demand","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6580, data_dir: "~/.stockyard", log_level: "info", product: "tokenauction",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "tokenauction_stats", description: "Get auction stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "tokenauction_proxy_status", description: "Check if the TokenAuction proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  canarydeploy: {
    binary: "canarydeploy",
    port: 6590,
    displayName: "CanaryDeploy",
    tagline: "Canary deployments for prompt/model changes",
    description: "Gradual rollout: 5%→25%→100%. Auto-promote if quality holds. Auto-rollback.",
    keywords: ["mcp","mcp-server","llm","canary","deployment","rollout","gradual","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6590, data_dir: "~/.stockyard", log_level: "info", product: "canarydeploy",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "canarydeploy_stats", description: "Get canary deployment stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "canarydeploy_proxy_status", description: "Check if the CanaryDeploy proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  playbackstudio: {
    binary: "playbackstudio",
    port: 6600,
    displayName: "PlaybackStudio",
    tagline: "Interactive playground for exploring logged interactions",
    description: "Advanced filters. Conversation threads. Side-by-side. Bulk actions.",
    keywords: ["mcp","mcp-server","llm","playground","explore","logs","interactive","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6600, data_dir: "~/.stockyard", log_level: "info", product: "playbackstudio",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "playbackstudio_stats", description: "Get exploration stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "playbackstudio_proxy_status", description: "Check if the PlaybackStudio proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },

  webhookforge: {
    binary: "webhookforge",
    port: 6610,
    displayName: "WebhookForge",
    tagline: "Visual builder for webhook→LLM→action pipelines",
    description: "Visual flow builder. Trigger→transform→LLM→condition→action. History.",
    keywords: ["mcp","mcp-server","llm","webhook","builder","visual","pipeline","proxy","stockyard","model-context-protocol","cursor","claude-desktop"],
    defaultConfig: {
      port: 6610, data_dir: "~/.stockyard", log_level: "info", product: "webhookforge",
      providers: { openai: { api_key: "${OPENAI_API_KEY}", base_url: "https://api.openai.com/v1" } },
    },
    tools: [
      { name: "webhookforge_stats", description: "Get pipeline stats.", inputSchema: { type: "object", properties: {} }, apiPath: "/api/stats" },
      { name: "webhookforge_proxy_status", description: "Check if the WebhookForge proxy is running and healthy.", inputSchema: { type: "object", properties: {} }, apiPath: "/health" },
    ],
  },
};

module.exports = { EXPANSION_PRODUCTS };
