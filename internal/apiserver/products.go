package apiserver

// Product represents a Stockyard product in the catalog.
type Product struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Tagline     string            `json:"tagline"`
	Category    string            `json:"category"`
	IsSuite     bool              `json:"is_suite,omitempty"`
	Pricing     map[string]int    `json:"pricing"`      // tier → cents/month
	StripePrices map[string]string `json:"stripe_prices"` // tier → Stripe price ID
}

// PricingTiers defines the price in cents per month for each tier.
var PricingTiers = map[string]map[string]int{
	"individual": {
		"starter": 900,   // $9/mo
		"pro":     2900,  // $29/mo
		"team":    7900,  // $79/mo
	},
	"suite": {
		"starter": 1900,  // $19/mo
		"pro":     5900,  // $59/mo
		"team":    14900, // $149/mo
	},
}

// Catalog returns the full product catalog.
// Stripe price IDs are set via environment variables: STRIPE_PRICE_{PRODUCT}_{TIER}
// e.g., STRIPE_PRICE_COSTCAP_PRO=price_1abc...
func Catalog() []Product {
	return []Product{
		// Suite
		{Slug: "stockyard", Name: "Stockyard Suite", Tagline: "The complete LLM proxy toolkit — 125 products.", Category: "suite", IsSuite: true, Pricing: PricingTiers["suite"]},

		// Original 7
		{Slug: "costcap", Name: "CostCap", Tagline: "Never get a surprise LLM bill again.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "llmcache", Name: "CacheLayer", Tagline: "Stop paying for the same answer twice.", Category: "performance", Pricing: PricingTiers["individual"]},
		{Slug: "jsonguard", Name: "StructuredShield", Tagline: "Guaranteed JSON from any LLM.", Category: "reliability", Pricing: PricingTiers["individual"]},
		{Slug: "routefall", Name: "FallbackRouter", Tagline: "When OpenAI goes down, your app doesn't.", Category: "reliability", Pricing: PricingTiers["individual"]},
		{Slug: "rateshield", Name: "RateShield", Tagline: "Rate limiting that actually works for LLMs.", Category: "security", Pricing: PricingTiers["individual"]},
		{Slug: "promptreplay", Name: "PromptReplay", Tagline: "Record, replay, debug every LLM call.", Category: "devex", Pricing: PricingTiers["individual"]},

		// Phase 1
		{Slug: "keypool", Name: "KeyPool", Tagline: "API key rotation on autopilot.", Category: "security", Pricing: PricingTiers["individual"]},
		{Slug: "promptguard", Name: "PromptGuard", Tagline: "PII redaction + injection detection.", Category: "security", Pricing: PricingTiers["individual"]},
		{Slug: "modelswitch", Name: "ModelSwitch", Tagline: "Route requests to the right model, every time.", Category: "routing", Pricing: PricingTiers["individual"]},
		{Slug: "evalgate", Name: "EvalGate", Tagline: "Quality scoring for every LLM response.", Category: "quality", Pricing: PricingTiers["individual"]},
		{Slug: "usagepulse", Name: "UsagePulse", Tagline: "Per-user token metering and billing.", Category: "cost", Pricing: PricingTiers["individual"]},

		// Phase 2
		{Slug: "promptpad", Name: "PromptPad", Tagline: "Version control for your prompts.", Category: "prompt", Pricing: PricingTiers["individual"]},
		{Slug: "tokentrim", Name: "TokenTrim", Tagline: "Fit more into your context window.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "batchqueue", Name: "BatchQueue", Tagline: "Async LLM queue with concurrency control.", Category: "performance", Pricing: PricingTiers["individual"]},
		{Slug: "multicall", Name: "MultiCall", Tagline: "Multi-model consensus in one call.", Category: "quality", Pricing: PricingTiers["individual"]},
		{Slug: "streamsnap", Name: "StreamSnap", Tagline: "Capture and replay SSE streams.", Category: "devex", Pricing: PricingTiers["individual"]},
		{Slug: "llmtap", Name: "LLMTap", Tagline: "Full analytics portal for your LLM stack.", Category: "observability", Pricing: PricingTiers["individual"]},
		{Slug: "contextpack", Name: "ContextPack", Tagline: "Poor man's RAG — inject files as context.", Category: "rag", Pricing: PricingTiers["individual"]},
		{Slug: "retrypilot", Name: "RetryPilot", Tagline: "Intelligent retry with model downgrade.", Category: "reliability", Pricing: PricingTiers["individual"]},

		// Phase 3 — Safety
		{Slug: "toxicfilter", Name: "ToxicFilter", Tagline: "Content moderation for LLM outputs.", Category: "safety", Pricing: PricingTiers["individual"]},
		{Slug: "hallucicheck", Name: "HalluciCheck", Tagline: "Catch hallucinations before users do.", Category: "safety", Pricing: PricingTiers["individual"]},
		{Slug: "guardrail", Name: "GuardRail", Tagline: "Keep your LLM on-topic.", Category: "safety", Pricing: PricingTiers["individual"]},
		{Slug: "compliancelog", Name: "ComplianceLog", Tagline: "Immutable audit trail for LLM interactions.", Category: "compliance", Pricing: PricingTiers["individual"]},
		{Slug: "agegate", Name: "AgeGate", Tagline: "Child safety middleware for LLM apps.", Category: "safety", Pricing: PricingTiers["individual"]},

		// Phase 3 — Cost
		{Slug: "promptslim", Name: "PromptSlim", Tagline: "Compress prompts 40-70% without losing meaning.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "outputcap", Name: "OutputCap", Tagline: "Stop paying for responses you don't need.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "tierdrop", Name: "TierDrop", Tagline: "Auto-downgrade models when burning cash.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "idlekill", Name: "IdleKill", Tagline: "Kill runaway LLM requests.", Category: "cost", Pricing: PricingTiers["individual"]},

		// Phase 3 — SaaS
		{Slug: "tenantwall", Name: "TenantWall", Tagline: "Per-tenant isolation for multi-tenant apps.", Category: "saas", Pricing: PricingTiers["individual"]},
		{Slug: "billsync", Name: "BillSync", Tagline: "Usage-based billing for your LLM SaaS.", Category: "saas", Pricing: PricingTiers["individual"]},
		{Slug: "whitelabel", Name: "WhiteLabel", Tagline: "Your brand on Stockyard's engine.", Category: "saas", Pricing: PricingTiers["individual"]},

		// Phase 3 — DevEx
		{Slug: "mockllm", Name: "MockLLM", Tagline: "Deterministic LLM responses for CI/CD.", Category: "devex", Pricing: PricingTiers["individual"]},
		{Slug: "diffprompt", Name: "DiffPrompt", Tagline: "Git-style diff for prompt changes.", Category: "devex", Pricing: PricingTiers["individual"]},
		{Slug: "llmbench", Name: "LLMBench", Tagline: "Benchmark any model on YOUR workload.", Category: "devex", Pricing: PricingTiers["individual"]},
		{Slug: "devproxy", Name: "DevProxy", Tagline: "Charles Proxy for LLM APIs.", Category: "devex", Pricing: PricingTiers["individual"]},

		// Phase 3 — Observability
		{Slug: "driftwatch", Name: "DriftWatch", Tagline: "Detect model behavior changes automatically.", Category: "observability", Pricing: PricingTiers["individual"]},
		{Slug: "tracelink", Name: "TraceLink", Tagline: "Distributed tracing for LLM chains.", Category: "observability", Pricing: PricingTiers["individual"]},
		{Slug: "alertpulse", Name: "AlertPulse", Tagline: "PagerDuty for your LLM stack.", Category: "observability", Pricing: PricingTiers["individual"]},
		{Slug: "regionroute", Name: "RegionRoute", Tagline: "Data residency routing for GDPR.", Category: "compliance", Pricing: PricingTiers["individual"]},

		// Phase 3 — Prompt
		{Slug: "approvalgate", Name: "ApprovalGate", Tagline: "Human approval for prompt changes.", Category: "prompt", Pricing: PricingTiers["individual"]},
		{Slug: "promptlint", Name: "PromptLint", Tagline: "Catch prompt anti-patterns.", Category: "prompt", Pricing: PricingTiers["individual"]},

		// Phase 3 — Use-Case
		{Slug: "chatmem", Name: "ChatMem", Tagline: "Persistent conversation memory.", Category: "usecase", Pricing: PricingTiers["individual"]},
		{Slug: "voicebridge", Name: "VoiceBridge", Tagline: "LLM middleware for voice pipelines.", Category: "usecase", Pricing: PricingTiers["individual"]},
		{Slug: "imageproxy", Name: "ImageProxy", Tagline: "Proxy for image generation APIs.", Category: "usecase", Pricing: PricingTiers["individual"]},
		{Slug: "embedcache", Name: "EmbedCache", Tagline: "Never compute the same embedding twice.", Category: "usecase", Pricing: PricingTiers["individual"]},
		{Slug: "agentguard", Name: "AgentGuard", Tagline: "Safety rails for autonomous agents.", Category: "usecase", Pricing: PricingTiers["individual"]},
		{Slug: "codefence", Name: "CodeFence", Tagline: "Validate LLM-generated code.", Category: "usecase", Pricing: PricingTiers["individual"]},
		{Slug: "langbridge", Name: "LangBridge", Tagline: "Multilingual translation middleware.", Category: "usecase", Pricing: PricingTiers["individual"]},

		// Phase 3 — Data
		{Slug: "feedbackloop", Name: "FeedbackLoop", Tagline: "Collect user feedback, close the loop.", Category: "data", Pricing: PricingTiers["individual"]},
		{Slug: "trainexport", Name: "TrainExport", Tagline: "Export conversations as fine-tuning datasets.", Category: "data", Pricing: PricingTiers["individual"]},
		{Slug: "synthgen", Name: "SynthGen", Tagline: "Generate synthetic training data.", Category: "data", Pricing: PricingTiers["individual"]},

		// Phase 3 — Provider
		{Slug: "anthrofit", Name: "AnthroFit", Tagline: "Use Claude with OpenAI SDKs.", Category: "provider", Pricing: PricingTiers["individual"]},
		{Slug: "geminishim", Name: "GeminiShim", Tagline: "Tame Gemini's quirks.", Category: "provider", Pricing: PricingTiers["individual"]},
		{Slug: "localsync", Name: "LocalSync", Tagline: "Blend local and cloud models.", Category: "provider", Pricing: PricingTiers["individual"]},

		// Phase 3 — Security
		{Slug: "secretscan", Name: "SecretScan", Tagline: "Catch API keys leaking in prompts.", Category: "security", Pricing: PricingTiers["individual"]},
		{Slug: "encryptvault", Name: "EncryptVault", Tagline: "E2E encryption for LLM payloads.", Category: "security", Pricing: PricingTiers["individual"]},
		{Slug: "ipfence", Name: "IPFence", Tagline: "IP allowlisting for LLM endpoints.", Category: "security", Pricing: PricingTiers["individual"]},

		// Phase 3 — Workflow
		{Slug: "chainforge", Name: "ChainForge", Tagline: "Multi-step LLM workflows as YAML.", Category: "workflow", Pricing: PricingTiers["individual"]},
		{Slug: "cronllm", Name: "CronLLM", Tagline: "Scheduled LLM tasks.", Category: "workflow", Pricing: PricingTiers["individual"]},
		{Slug: "webhookrelay", Name: "WebhookRelay", Tagline: "Trigger LLM calls from webhooks.", Category: "workflow", Pricing: PricingTiers["individual"]},

		// Phase 3 — Niche
		{Slug: "tokenmarket", Name: "TokenMarket", Tagline: "Dynamic API capacity reallocation.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "abrouter", Name: "ABRouter", Tagline: "A/B test any LLM variable.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "contextwindow", Name: "ContextWindow", Tagline: "Visual context window debugger.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "maskmode", Name: "MaskMode", Tagline: "Demo mode with realistic fake data.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "llmsync", Name: "LLMSync", Tagline: "Replicate config across environments.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "clustermode", Name: "ClusterMode", Tagline: "Multi-instance with shared state.", Category: "niche", Pricing: PricingTiers["individual"]},

		// Phase 4 — Structured Data
		{Slug: "extractml", Name: "ExtractML", Tagline: "Extract structured data from LLM prose.", Category: "structured", Pricing: PricingTiers["individual"]},
		{Slug: "tableforge", Name: "TableForge", Tagline: "LLM-powered table generation.", Category: "structured", Pricing: PricingTiers["individual"]},

		// Phase 4 — Tools
		{Slug: "toolrouter", Name: "ToolRouter", Tagline: "Manage and route LLM function calls.", Category: "tools", Pricing: PricingTiers["individual"]},
		{Slug: "toolshield", Name: "ToolShield", Tagline: "Validate tool calls before execution.", Category: "tools", Pricing: PricingTiers["individual"]},
		{Slug: "toolmock", Name: "ToolMock", Tagline: "Mock tool responses for testing.", Category: "tools", Pricing: PricingTiers["individual"]},

		// Phase 4 — Auth
		{Slug: "authgate", Name: "AuthGate", Tagline: "API key management for YOUR users.", Category: "auth", Pricing: PricingTiers["individual"]},
		{Slug: "scopeguard", Name: "ScopeGuard", Tagline: "Fine-grained API key permissions.", Category: "auth", Pricing: PricingTiers["individual"]},

		// Phase 4 — Multimodal
		{Slug: "visionproxy", Name: "VisionProxy", Tagline: "Proxy for vision/image APIs.", Category: "multimodal", Pricing: PricingTiers["individual"]},
		{Slug: "audioproxy", Name: "AudioProxy", Tagline: "Proxy for speech-to-text/TTS.", Category: "multimodal", Pricing: PricingTiers["individual"]},
		{Slug: "docparse", Name: "DocParse", Tagline: "Preprocess documents for LLMs.", Category: "multimodal", Pricing: PricingTiers["individual"]},
		{Slug: "framegrab", Name: "FrameGrab", Tagline: "Video frame extraction for vision LLMs.", Category: "multimodal", Pricing: PricingTiers["individual"]},

		// Phase 4 — Sessions
		{Slug: "sessionstore", Name: "SessionStore", Tagline: "Managed conversation sessions.", Category: "sessions", Pricing: PricingTiers["individual"]},
		{Slug: "convofork", Name: "ConvoFork", Tagline: "Branch conversations, try different paths.", Category: "sessions", Pricing: PricingTiers["individual"]},
		{Slug: "slotfill", Name: "SlotFill", Tagline: "Form-filling conversation engine.", Category: "sessions", Pricing: PricingTiers["individual"]},

		// Phase 4 — Caching
		{Slug: "semanticcache", Name: "SemanticCache", Tagline: "Cache hits for similar prompts.", Category: "caching", Pricing: PricingTiers["individual"]},
		{Slug: "partialcache", Name: "PartialCache", Tagline: "Cache reusable prompt prefixes.", Category: "caching", Pricing: PricingTiers["individual"]},
		{Slug: "streamcache", Name: "StreamCache", Tagline: "Cache streaming responses with timing.", Category: "caching", Pricing: PricingTiers["individual"]},

		// Phase 4 — Prompt Mgmt
		{Slug: "promptchain", Name: "PromptChain", Tagline: "Composable prompt building blocks.", Category: "prompt", Pricing: PricingTiers["individual"]},
		{Slug: "promptfuzz", Name: "PromptFuzz", Tagline: "Fuzz-test prompts with adversarial inputs.", Category: "prompt", Pricing: PricingTiers["individual"]},
		{Slug: "promptmarket", Name: "PromptMarket", Tagline: "Community prompt library.", Category: "prompt", Pricing: PricingTiers["individual"]},

		// Phase 4 — Cost Intel
		{Slug: "costpredict", Name: "CostPredict", Tagline: "Predict request cost before sending.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "costmap", Name: "CostMap", Tagline: "Multi-dimensional cost attribution.", Category: "cost", Pricing: PricingTiers["individual"]},
		{Slug: "spotprice", Name: "SpotPrice", Tagline: "Real-time model pricing intelligence.", Category: "cost", Pricing: PricingTiers["individual"]},

		// Phase 4 — Testing
		{Slug: "loadforge", Name: "LoadForge", Tagline: "Load test your LLM stack.", Category: "testing", Pricing: PricingTiers["individual"]},
		{Slug: "snapshottest", Name: "SnapshotTest", Tagline: "Snapshot testing for LLM outputs.", Category: "testing", Pricing: PricingTiers["individual"]},
		{Slug: "chaosllm", Name: "ChaosLLM", Tagline: "Chaos engineering for LLMs.", Category: "testing", Pricing: PricingTiers["individual"]},

		// Phase 4 — Compliance
		{Slug: "datamap", Name: "DataMap", Tagline: "GDPR data flow mapping.", Category: "compliance", Pricing: PricingTiers["individual"]},
		{Slug: "consentgate", Name: "ConsentGate", Tagline: "User consent management for AI.", Category: "compliance", Pricing: PricingTiers["individual"]},
		{Slug: "retentionwipe", Name: "RetentionWipe", Tagline: "Automated data retention and deletion.", Category: "compliance", Pricing: PricingTiers["individual"]},
		{Slug: "policyengine", Name: "PolicyEngine", Tagline: "Codify AI governance as enforceable rules.", Category: "compliance", Pricing: PricingTiers["individual"]},

		// Phase 4 — Streaming
		{Slug: "streamsplit", Name: "StreamSplit", Tagline: "Fork streams to multiple destinations.", Category: "streaming", Pricing: PricingTiers["individual"]},
		{Slug: "streamthrottle", Name: "StreamThrottle", Tagline: "Control streaming speed for UX.", Category: "streaming", Pricing: PricingTiers["individual"]},
		{Slug: "streamtransform", Name: "StreamTransform", Tagline: "Transform responses mid-stream.", Category: "streaming", Pricing: PricingTiers["individual"]},

		// Phase 4 — Provider
		{Slug: "modelalias", Name: "ModelAlias", Tagline: "Abstract away model names.", Category: "provider", Pricing: PricingTiers["individual"]},
		{Slug: "paramnorm", Name: "ParamNorm", Tagline: "Normalize params across providers.", Category: "provider", Pricing: PricingTiers["individual"]},
		{Slug: "quotasync", Name: "QuotaSync", Tagline: "Track provider rate limits.", Category: "provider", Pricing: PricingTiers["individual"]},
		{Slug: "errornorm", Name: "ErrorNorm", Tagline: "Normalize errors across providers.", Category: "provider", Pricing: PricingTiers["individual"]},

		// Phase 4 — Analytics
		{Slug: "cohorttrack", Name: "CohortTrack", Tagline: "User cohort analytics.", Category: "analytics", Pricing: PricingTiers["individual"]},
		{Slug: "promptrank", Name: "PromptRank", Tagline: "Rank prompts by ROI.", Category: "analytics", Pricing: PricingTiers["individual"]},
		{Slug: "anomalyradar", Name: "AnomalyRadar", Tagline: "ML-powered anomaly detection.", Category: "analytics", Pricing: PricingTiers["individual"]},

		// Phase 4 — DevWorkflow
		{Slug: "envsync", Name: "EnvSync", Tagline: "Sync configs across environments.", Category: "devworkflow", Pricing: PricingTiers["individual"]},
		{Slug: "proxylog", Name: "ProxyLog", Tagline: "Structured logging for proxy decisions.", Category: "devworkflow", Pricing: PricingTiers["individual"]},
		{Slug: "clidash", Name: "CliDash", Tagline: "Terminal dashboard for your LLM stack.", Category: "devworkflow", Pricing: PricingTiers["individual"]},

		// Phase 4 — Specialized
		{Slug: "embedrouter", Name: "EmbedRouter", Tagline: "Smart routing for embeddings.", Category: "specialized", Pricing: PricingTiers["individual"]},
		{Slug: "finetunetrack", Name: "FineTuneTrack", Tagline: "Monitor fine-tuned model performance.", Category: "specialized", Pricing: PricingTiers["individual"]},
		{Slug: "agentreplay", Name: "AgentReplay", Tagline: "Replay agent sessions step-by-step.", Category: "specialized", Pricing: PricingTiers["individual"]},
		{Slug: "summarizegate", Name: "SummarizeGate", Tagline: "Auto-summarize long contexts.", Category: "specialized", Pricing: PricingTiers["individual"]},
		{Slug: "codelang", Name: "CodeLang", Tagline: "Language-aware code validation.", Category: "specialized", Pricing: PricingTiers["individual"]},
		{Slug: "personaswitch", Name: "PersonaSwitch", Tagline: "Hot-swap AI personalities.", Category: "specialized", Pricing: PricingTiers["individual"]},

		// Phase 4 — Infrastructure
		{Slug: "warmpool", Name: "WarmPool", Tagline: "Pre-warm model connections.", Category: "infrastructure", Pricing: PricingTiers["individual"]},
		{Slug: "edgecache", Name: "EdgeCache", Tagline: "CDN-like caching for LLM responses.", Category: "infrastructure", Pricing: PricingTiers["individual"]},
		{Slug: "queuepriority", Name: "QueuePriority", Tagline: "VIP users first.", Category: "infrastructure", Pricing: PricingTiers["individual"]},
		{Slug: "geoprice", Name: "GeoPrice", Tagline: "PPP-adjusted regional pricing.", Category: "infrastructure", Pricing: PricingTiers["individual"]},

		// Phase 4 — Niche
		{Slug: "tokenauction", Name: "TokenAuction", Tagline: "Dynamic pricing based on demand.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "canarydeploy", Name: "CanaryDeploy", Tagline: "Canary deployments for prompt changes.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "playbackstudio", Name: "PlaybackStudio", Tagline: "Explore logged interactions.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "webhookforge", Name: "WebhookForge", Tagline: "Visual webhook→LLM pipelines.", Category: "niche", Pricing: PricingTiers["individual"]},
		{Slug: "mirrortest", Name: "MirrorTest", Tagline: "Shadow test models on live traffic.", Category: "niche", Pricing: PricingTiers["individual"]},
	}
}

// ProductBySlug returns a product by slug.
func ProductBySlug(slug string) *Product {
	for _, p := range Catalog() {
		if p.Slug == slug {
			return &p
		}
	}
	return nil
}

// CatalogCount returns the number of products (should be 125 + suite = 126).
func CatalogCount() int {
	return len(Catalog())
}
