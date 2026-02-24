package config

import "time"

// DefaultConfig returns the default configuration for a given product.
func DefaultConfig(product string) *Config {
	base := &Config{
		DataDir:  "~/.stockyard",
		LogLevel: "info",
		Product:  product,
		Providers: map[string]ProviderConfig{
			"openai": {
				APIKey:     "${OPENAI_API_KEY}",
				BaseURL:    "https://api.openai.com/v1",
				Timeout:    Duration{30 * time.Second},
				MaxRetries: 3,
			},
		},
		Projects: map[string]ProjectConfig{
			"default": {
				Provider: "openai",
				Model:    "gpt-4o-mini",
			},
		},
		Logging: LoggingConfig{
			RetentionDays: 7,
			MaxBodySize:   50000,
		},
	}

	switch product {
	case "costcap":
		base.Port = 4100
		base.Logging.StoreBodies = false
		base.Projects["default"] = ProjectConfig{
			Provider: "openai",
			Model:    "gpt-4o-mini",
			Caps:     CapsConfig{Daily: 5.00, Monthly: 50.00},
			Alerts:   AlertsConfig{Thresholds: []float64{0.5, 0.8, 1.0}},
		}

	case "llmcache":
		base.Port = 4101
		base.Logging.StoreBodies = true
		base.Cache = CacheConfig{
			Enabled:    true,
			Strategy:   "exact",
			TTL:        Duration{1 * time.Hour},
			MaxEntries: 10000,
		}

	case "jsonguard":
		base.Port = 4102
		base.Logging.StoreBodies = true
		base.Validation = ValidationConfig{
			Enabled:    true,
			MaxRetries: 3,
			Schemas:    make(map[string]any),
		}

	case "routefall":
		base.Port = 4103
		base.Logging.StoreBodies = false
		base.Failover = FailoverConfig{
			Enabled:  true,
			Strategy: "priority",
			Providers: []string{"openai", "anthropic", "groq"},
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: 5,
				RecoveryTimeout:  Duration{60 * time.Second},
			},
		}

	case "rateshield":
		base.Port = 4104
		base.Logging.StoreBodies = false
		base.RateLimit = RateLimitConfig{
			Enabled: true,
			Default: RateLimitRule{
				RequestsPerMinute: 60,
				RequestsPerHour:   1000,
				Burst:             10,
			},
			PerIP:   true,
			PerUser: true,
			Abuse: AbuseConfig{
				Enabled:            true,
				DuplicateThreshold: 10,
			},
		}

	case "promptreplay":
		base.Port = 4105
		base.Logging.StoreBodies = true
		base.Logging.MaxBodySize = 100000

	case "stockyard":
		base.Port = 4200
		base.Logging.StoreBodies = true
		base.Cache = CacheConfig{
			Enabled:    true,
			Strategy:   "exact",
			TTL:        Duration{1 * time.Hour},
			MaxEntries: 10000,
		}
		base.Validation = ValidationConfig{
			Enabled:    true,
			MaxRetries: 3,
			Schemas:    make(map[string]any),
		}
		base.Failover = FailoverConfig{
			Enabled:  true,
			Strategy: "priority",
			Providers: []string{"openai", "anthropic", "groq"},
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: 5,
				RecoveryTimeout:  Duration{60 * time.Second},
			},
		}
		base.RateLimit = RateLimitConfig{
			Enabled: true,
			Default: RateLimitRule{
				RequestsPerMinute: 60,
				RequestsPerHour:   1000,
				Burst:             10,
			},
			PerIP:   true,
			PerUser: true,
		}
		base.PromptGuard = PromptGuardConfig{
			Enabled: true,
			PII: PIIConfig{
				Mode:     "redact",
				Patterns: []string{"email", "ssn", "phone", "credit_card"},
			},
			Injection: InjectionConfig{
				Enabled:     true,
				Sensitivity: "medium",
				Action:      "warn",
			},
		}
		base.UsagePulse = UsagePulseConfig{
			Enabled:      true,
			Dimensions:   []string{"user", "project", "model"},
			ExportFormat: "json",
		}
		base.TokenTrim = TokenTrimConfig{
			Enabled:      true,
			DefaultStrat: "middle-out",
			SafetyMargin: 500,
			Protect:      []string{"system"},
		}
		base.ContextPack = ContextPackConfig{
			Enabled: true,
			Injection: ContextInjection{
				Position:  "before_user",
				MaxTokens: 2000,
			},
		}
		base.PromptPad = PromptPadConfig{
			Enabled:   true,
			Storage:   "sqlite",
			APIPrefix: "/api/prompts",
		}
		base.MultiCall = MultiCallConfig{
			Enabled:  true,
			Fallback: "fastest_available",
		}
		base.StreamSnap = StreamSnapConfig{
			Enabled: true,
			Metrics: StreamMetrics{TTFT: true, TPS: true, CompletionCheck: true},
			Replay:  StreamReplay{Enabled: true},
		}
		base.LLMTap = LLMTapConfig{
			Enabled:     true,
			Percentiles: []int{50, 95, 99},
			Granularity: "hourly",
		}
		base.RetryPilot = RetryPilotConfig{
			Enabled:    true,
			MaxRetries: 3,
			Backoff:    "exponential",
			Jitter:     "full",
			CircuitBreaker: RetryCircuitBreaker{
				FailureThreshold: 5,
				HalfOpenRequests: 2,
			},
			Downgrade: RetryDowngrade{
				Enabled:       true,
				AfterFailures: 2,
				DowngradeMap: map[string]string{
					"gpt-4o": "gpt-4o-mini",
				},
			},
		}
		base.ToxicFilter = ToxicFilterConfig{
			Enabled:    true,
			ScanOutput: true,
			Action:     "flag",
			Categories: []ToxicCategory{
				{Name: "harmful", Enabled: true, Action: "block"},
				{Name: "hate_speech", Enabled: true, Action: "block"},
				{Name: "self_harm", Enabled: true, Action: "block"},
			},
		}
		base.ComplianceLog = ComplianceLogConfig{
			Enabled:       true,
			HashAlgorithm: "sha256",
			RetentionDays: 90,
			ExportFormats: []string{"json", "csv"},
			IncludeBodies: true,
			MaxBodySize:   50000,
		}
		base.SecretScan = SecretScanConfig{
			Enabled:     true,
			ScanInput:   true,
			ScanOutput:  true,
			Action:      "redact",
			MaskPreview: true,
			Patterns:    []string{"aws_key", "aws_secret", "github_pat", "openai_key", "anthropic_key", "stripe_key", "private_key"},
		}
		base.TraceLink = TraceLinkConfig{
			Enabled:      true,
			SampleRate:   1.0,
			PropagateW3C: true,
			ServiceName:  "stockyard",
			MaxSpans:     10000,
		}
		base.IPFence = IPFenceConfig{
			Enabled:    true,
			Mode:       "denylist",
			Action:     "block",
			LogBlocked: true,
			TrustProxy: true,
		}
		base.EmbedCache = EmbedCacheConfig{
			Enabled:    true,
			MaxEntries: 100000,
			TTL:        Duration{7 * 24 * time.Hour},
		}
		base.AnthroFit = AnthroFitConfig{
			Enabled:          true,
			SystemPromptMode: "auto",
			ToolTranslation:  true,
			StreamNormalize:  true,
			MaxTokensDefault: 4096,
			CacheControl:     true,
		}
		base.AlertPulse = AlertPulseConfig{
			Enabled:        true,
			WindowDuration: Duration{5 * time.Minute},
			Cooldown:       Duration{5 * time.Minute},
			Rules: []AlertRule{
				{Name: "high_error_rate", Metric: "error_rate", Threshold: 25, Channel: "webhook"},
				{Name: "slow_p95", Metric: "latency_p95", Threshold: 5000, Channel: "webhook"},
			},
		}
		base.ChatMem = ChatMemConfig{
			Enabled:      true,
			Strategy:     "sliding_window",
			MaxMessages:  50,
			InjectMemory: true,
			SessionTTL:   Duration{1 * time.Hour},
		}
		base.MockLLM = MockLLMConfig{
			Enabled:         false,
			DefaultResponse: "Mock response — no fixture matched.",
			Passthrough:     true,
		}
		base.TenantWall = TenantWallConfig{
			Enabled:              true,
			RequireTenant:        false,
			UseProjectAsTenant:   true,
			WindowDuration:       Duration{1 * time.Hour},
			DefaultMaxRequests:   1000,
			DefaultMaxSpend:      10.0,
			DefaultAllowedModels: []string{"*"},
		}
		base.IdleKill = IdleKillConfig{
			Enabled:             true,
			MaxDuration:         Duration{120 * time.Second},
			MaxTokensPerRequest: 50000,
			MaxCostPerRequest:   0.50,
			LoopDetection:       true,
			LoopWindow:          Duration{60 * time.Second},
			LoopThreshold:       5,
		}

	case "keypool":
		base.Port = 4700
		base.Logging.StoreBodies = false
		base.KeyPool = KeyPoolConfig{
			Enabled:  true,
			Strategy: "least-used",
			Cooldown: Duration{60 * time.Second},
			Keys: []PooledKeyEntry{
				{Name: "primary", Key: "${OPENAI_KEY_1}", Provider: "openai", Weight: 2},
				{Name: "secondary", Key: "${OPENAI_KEY_2}", Provider: "openai", Weight: 1},
				{Name: "burst", Key: "${OPENAI_KEY_3}", Provider: "openai", Weight: 1},
			},
		}

	case "promptguard":
		base.Port = 4701
		base.Logging.StoreBodies = true
		base.PromptGuard = PromptGuardConfig{
			Enabled: true,
			PII: PIIConfig{
				Mode:     "redact-restore",
				Patterns: []string{"email", "ssn", "phone", "credit_card"},
			},
			Injection: InjectionConfig{
				Enabled:     true,
				Sensitivity: "medium",
				Action:      "block",
			},
		}

	case "modelswitch":
		base.Port = 4702
		base.Logging.StoreBodies = false
		base.ModelSwitch = ModelSwitchConfig{
			Enabled: true,
			Default: "gpt-4o-mini",
			Rules: []ModelRouteRule{
				{
					Name:      "large-context",
					Condition: "token_count",
					Operator:  "gt",
					Value:     "4000",
					Model:     "gpt-4o",
					Weight:    100,
				},
				{
					Name:      "simple-queries",
					Condition: "token_count",
					Operator:  "lt",
					Value:     "500",
					Model:     "gpt-4o-mini",
					Weight:    100,
				},
			},
		}

	case "evalgate":
		base.Port = 4703
		base.Logging.StoreBodies = true
		base.EvalGate = EvalGateConfig{
			Enabled:     true,
			RetryBudget: 2,
			Validators: []ValidatorConfig{
				{Name: "not_empty", Action: "retry"},
				{Name: "min_length", Params: "10", Action: "retry"},
			},
		}

	case "usagepulse":
		base.Port = 4704
		base.Logging.StoreBodies = false
		base.UsagePulse = UsagePulseConfig{
			Enabled:      true,
			Dimensions:   []string{"user", "project", "model", "feature"},
			ExportFormat: "json",
		}

	// ── Phase 2 Products ──

	case "promptpad":
		base.Port = 4800
		base.Logging.StoreBodies = true
		base.PromptPad = PromptPadConfig{
			Enabled:   true,
			Storage:   "sqlite",
			APIPrefix: "/api/prompts",
		}

	case "tokentrim":
		base.Port = 4900
		base.Logging.StoreBodies = false
		base.TokenTrim = TokenTrimConfig{
			Enabled:      true,
			DefaultStrat: "middle-out",
			SafetyMargin: 500,
			Models: map[string]TrimModel{
				"gpt-4o":      {MaxContext: 128000, Strategy: "priority"},
				"gpt-4o-mini": {MaxContext: 128000, Strategy: "middle-out"},
			},
			Protect: []string{"system"},
		}

	case "batchqueue":
		base.Port = 5000
		base.Logging.StoreBodies = true
		base.BatchQueue = BatchQueueConfig{
			Enabled: true,
			Concurrency: BatchConcurrency{
				Default: 5,
				PerProvider: map[string]int{
					"openai":    10,
					"anthropic": 5,
					"groq":      20,
				},
			},
			Retry: BatchRetryConfig{
				MaxAttempts: 3,
				Backoff:     "exponential",
			},
			Delivery: BatchDelivery{
				Mode: "poll",
			},
			Priorities: []string{"urgent", "normal", "batch"},
		}

	case "multicall":
		base.Port = 5100
		base.Logging.StoreBodies = true
		base.MultiCall = MultiCallConfig{
			Enabled:  true,
			Fallback: "fastest_available",
		}

	case "streamsnap":
		base.Port = 5200
		base.Logging.StoreBodies = false
		base.StreamSnap = StreamSnapConfig{
			Enabled: true,
			Metrics: StreamMetrics{
				TTFT:            true,
				TPS:             true,
				CompletionCheck: true,
			},
			Replay: StreamReplay{
				Enabled: true,
			},
		}

	case "llmtap":
		base.Port = 5300
		base.Logging.StoreBodies = false
		base.LLMTap = LLMTapConfig{
			Enabled:     true,
			Percentiles: []int{50, 95, 99},
			Granularity: "hourly",
		}

	case "contextpack":
		base.Port = 5400
		base.Logging.StoreBodies = false
		base.ContextPack = ContextPackConfig{
			Enabled: true,
			Injection: ContextInjection{
				Position:  "before_user",
				MaxTokens: 2000,
				Template:  "Relevant context:\n---\n{{context}}\n---",
			},
		}

	case "retrypilot":
		base.Port = 5500
		base.Logging.StoreBodies = false
		base.RetryPilot = RetryPilotConfig{
			Enabled:    true,
			MaxRetries: 3,
			Backoff:    "exponential",
			Jitter:     "full",
			CircuitBreaker: RetryCircuitBreaker{
				FailureThreshold: 5,
				HalfOpenRequests: 2,
			},
			Downgrade: RetryDowngrade{
				Enabled:       true,
				AfterFailures: 2,
				DowngradeMap: map[string]string{
					"gpt-4o":           "gpt-4o-mini",
					"claude-sonnet-4-20250514": "claude-haiku-4-5-20251001",
				},
			},
			Budget: RetryBudget{
				MaxPerMinute: 30,
			},
		}

	// ── Phase 3 Products ──

	case "toxicfilter":
		base.Port = 5600
		base.Logging.StoreBodies = true
		base.ToxicFilter = ToxicFilterConfig{
			Enabled:    true,
			ScanInput:  false,
			ScanOutput: true,
			Action:     "flag",
			Categories: []ToxicCategory{
				{Name: "harmful", Enabled: true, Action: "block"},
				{Name: "hate_speech", Enabled: true, Action: "block"},
				{Name: "violence", Enabled: true, Action: "flag"},
				{Name: "self_harm", Enabled: true, Action: "block"},
				{Name: "sexual", Enabled: true, Action: "redact"},
				{Name: "profanity", Enabled: true, Action: "flag"},
			},
		}

	case "compliancelog":
		base.Port = 5610
		base.Logging.StoreBodies = true
		base.ComplianceLog = ComplianceLogConfig{
			Enabled:         true,
			HashAlgorithm:   "sha256",
			RetentionDays:   365,
			ExportFormats:   []string{"json", "csv", "soc2"},
			IncludeHeaders:  true,
			IncludeBodies:   true,
			MaxBodySize:     100000,
			VerifyOnStartup: true,
		}

	case "secretscan":
		base.Port = 5620
		base.Logging.StoreBodies = true
		base.SecretScan = SecretScanConfig{
			Enabled:     true,
			ScanInput:   true,
			ScanOutput:  true,
			Action:      "redact",
			MaskPreview: true,
			Patterns: []string{
				"aws_key", "aws_secret", "github_pat", "github_token",
				"stripe_key", "openai_key", "anthropic_key",
				"gcp_key", "azure_key", "slack_token",
				"jwt", "private_key", "generic_secret",
			},
		}

	case "tracelink":
		base.Port = 5630
		base.Logging.StoreBodies = false
		base.TraceLink = TraceLinkConfig{
			Enabled:      true,
			SampleRate:   1.0,
			PropagateW3C: true,
			ServiceName:  "stockyard",
			MaxSpans:     10000,
		}

	case "ipfence":
		base.Port = 5690
		base.Logging.StoreBodies = false
		base.IPFence = IPFenceConfig{
			Enabled:    true,
			Mode:       "allowlist",
			Action:     "block",
			Allowlist:  []string{"127.0.0.1/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128"},
			TrustProxy: false,
			LogBlocked: true,
		}

	case "embedcache":
		base.Port = 5700
		base.Logging.StoreBodies = false
		base.EmbedCache = EmbedCacheConfig{
			Enabled:    true,
			MaxEntries: 100000,
			TTL:        Duration{7 * 24 * time.Hour},
		}

	case "anthrofit":
		base.Port = 5710
		base.Logging.StoreBodies = true
		base.AnthroFit = AnthroFitConfig{
			Enabled:          true,
			SystemPromptMode: "auto",
			ToolTranslation:  true,
			StreamNormalize:  true,
			MaxTokensDefault: 4096,
			CacheControl:     true,
		}

	case "alertpulse":
		base.Port = 5640
		base.Logging.StoreBodies = true
		base.AlertPulse = AlertPulseConfig{
			Enabled:        true,
			WindowDuration: Duration{5 * time.Minute},
			Cooldown:       Duration{5 * time.Minute},
			Rules: []AlertRule{
				{Name: "high_error_rate", Metric: "error_rate", Threshold: 25, Channel: "webhook"},
				{Name: "slow_p95", Metric: "latency_p95", Threshold: 5000, Channel: "webhook"},
				{Name: "cost_spike", Metric: "cost_per_min", Threshold: 1.0, Channel: "webhook"},
			},
		}

	case "chatmem":
		base.Port = 5650
		base.Logging.StoreBodies = true
		base.ChatMem = ChatMemConfig{
			Enabled:      true,
			Strategy:     "sliding_window",
			MaxMessages:  50,
			InjectMemory: true,
			SessionTTL:   Duration{1 * time.Hour},
		}

	case "mockllm":
		base.Port = 5660
		base.Logging.StoreBodies = true
		base.MockLLM = MockLLMConfig{
			Enabled:         true,
			DefaultResponse: "This is a mock response. No fixture matched your request.",
			Passthrough:     false,
			Fixtures: []MockFixture{
				{Name: "greeting", MatchType: "contains", Pattern: "hello", Response: "Hello! This is a mock response from MockLLM."},
				{Name: "catchall", MatchType: "any", Response: "MockLLM caught this request. Define fixtures to customize responses."},
			},
		}

	case "tenantwall":
		base.Port = 5670
		base.Logging.StoreBodies = true
		base.TenantWall = TenantWallConfig{
			Enabled:              true,
			RequireTenant:        true,
			UseProjectAsTenant:   true,
			WindowDuration:       Duration{1 * time.Hour},
			DefaultMaxRequests:   1000,
			DefaultMaxSpend:      10.0,
			DefaultAllowedModels: []string{"*"},
		}

	case "idlekill":
		base.Port = 5680
		base.Logging.StoreBodies = true
		base.IdleKill = IdleKillConfig{
			Enabled:             true,
			MaxDuration:         Duration{120 * time.Second},
			MaxTokensPerRequest: 50000,
			MaxCostPerRequest:   0.50,
			LoopDetection:       true,
			LoopWindow:          Duration{60 * time.Second},
			LoopThreshold:       5,
		}

	case "agentguard":
		base.Port = 5690
		base.Logging.StoreBodies = true
		base.AgentGuard = AgentGuardConfig{
			Enabled: true, MaxCalls: 100, MaxCost: 5.0,
			MaxDuration: Duration{30 * time.Minute}, SessionHeader: "X-Agent-Session",
		}

	case "codefence":
		base.Port = 5700
		base.Logging.StoreBodies = true
		base.CodeFence = CodeFenceConfig{Enabled: true, MaxComplexity: 0, Action: "flag"}

	case "hallucicheck":
		base.Port = 5710
		base.Logging.StoreBodies = true
		base.HalluciCheck = HalluciCheckConfig{Enabled: true, CheckURLs: true, CheckEmails: true, Action: "flag"}

	case "tierdrop":
		base.Port = 5720
		base.Logging.StoreBodies = true
		base.TierDrop = TierDropConfig{Enabled: true, Tiers: []TierDropTier{
			{Threshold: 0.8, Model: "gpt-4o-mini"},
			{Threshold: 0.95, Model: "gpt-3.5-turbo"},
		}}

	case "driftwatch":
		base.Port = 5730
		base.Logging.StoreBodies = true
		base.DriftWatch = DriftWatchConfig{Enabled: true, DriftThreshold: 50.0}

	case "feedbackloop":
		base.Port = 5740
		base.Logging.StoreBodies = true
		base.FeedbackLoop = FeedbackLoopConfig{Enabled: true, Endpoint: "/api/feedback"}

	case "abrouter":
		base.Port = 5750
		base.Logging.StoreBodies = true
		base.ABRouter = ABRouterConfig{Enabled: true}

	case "guardrail":
		base.Port = 5760
		base.Logging.StoreBodies = true
		base.GuardRail = GuardRailConfig{Enabled: true, FallbackMsg: "I can only help with topics within my designated scope."}

	case "geminishim":
		base.Port = 5770
		base.Logging.StoreBodies = true
		base.GeminiShim = GeminiShimConfig{Enabled: true, AutoRetrySafety: true, NormalizeTokens: true}

	case "localsync":
		base.Port = 5780
		base.Logging.StoreBodies = true
		base.LocalSync = LocalSyncConfig{Enabled: true, LocalEndpoint: "http://localhost:11434", FallbackToCloud: true}

	case "devproxy":
		base.Port = 5790
		base.Logging.StoreBodies = true
		base.DevProxy = DevProxyConfig{Enabled: true, LogHeaders: true, LogBodies: true}

	case "promptslim":
		base.Port = 5800
		base.Logging.StoreBodies = true
		base.PromptSlim = PromptSlimConfig{Enabled: true, Aggressiveness: 0.3}

	case "promptlint":
		base.Port = 5810
		base.Logging.StoreBodies = true
		base.PromptLint = PromptLintConfig{Enabled: true, BlockOnFail: false}

	case "approvalgate":
		base.Port = 5820
		base.Logging.StoreBodies = true
		base.ApprovalGate = ApprovalGateConfig{Enabled: true}

	case "outputcap":
		base.Port = 5830
		base.Logging.StoreBodies = true
		base.OutputCap = OutputCapConfig{Enabled: true, MaxChars: 4000}

	case "agegate":
		base.Port = 5840
		base.Logging.StoreBodies = true
		base.AgeGate = AgeGateConfig{Enabled: true, Tier: "child"}

	case "voicebridge":
		base.Port = 5850
		base.Logging.StoreBodies = true
		base.VoiceBridge = VoiceBridgeConfig{Enabled: true, MaxLength: 500}

	case "imageproxy":
		base.Port = 5860
		base.Logging.StoreBodies = true
		base.ImageProxy = ImageProxyConfig{Enabled: true, CacheEnabled: true}

	case "langbridge":
		base.Port = 5870
		base.Logging.StoreBodies = true
		base.LangBridge = LangBridgeConfig{Enabled: true, TargetLang: "en"}

	case "contextwindow":
		base.Port = 5880
		base.Logging.StoreBodies = true
		base.ContextWindow = ContextWindowConfig{Enabled: true}

	case "regionroute":
		base.Port = 5890
		base.Logging.StoreBodies = true
		base.RegionRoute = RegionRouteConfig{Enabled: true}

	case "chainforge":
		base.Port = 5900
		base.Logging.StoreBodies = true
		base.ChainForge = ChainForgeConfig{Enabled: true}

	case "cronllm":
		base.Port = 5910
		base.Logging.StoreBodies = true
		base.CronLLM = CronLLMConfig{Enabled: true}

	case "webhookrelay":
		base.Port = 5920
		base.Logging.StoreBodies = true
		base.WebhookRelay = WebhookRelayConfig{Enabled: true}

	case "billsync":
		base.Port = 5930
		base.Logging.StoreBodies = true
		base.BillSync = BillSyncConfig{Enabled: true, MarkupPct: 20.0, Currency: "USD"}

	case "whitelabel":
		base.Port = 5940
		base.Logging.StoreBodies = true
		base.WhiteLabel = WhiteLabelConfig{Enabled: true, BrandName: "My Platform"}

	case "trainexport":
		base.Port = 5950
		base.Logging.StoreBodies = true
		base.TrainExport = TrainExportConfig{Enabled: true, Format: "openai_jsonl", MaxPairs: 100000}

	case "synthgen":
		base.Port = 5960
		base.Logging.StoreBodies = true
		base.SynthGen = SynthGenConfig{Enabled: true, BatchSize: 10}

	case "diffprompt":
		base.Port = 5970
		base.Logging.StoreBodies = true
		base.DiffPrompt = DiffPromptConfig{Enabled: true}

	case "llmbench":
		base.Port = 5980
		base.Logging.StoreBodies = true
		base.LLMBench = LLMBenchConfig{Enabled: true}

	case "maskmode":
		base.Port = 5990
		base.Logging.StoreBodies = true
		base.MaskMode = MaskModeConfig{Enabled: true, MaskNames: true, MaskEmail: true, MaskPhone: true}

	case "tokenmarket":
		base.Port = 6000
		base.Logging.StoreBodies = true
		base.TokenMarket = TokenMarketConfig{Enabled: true}

	case "llmsync":
		base.Port = 6010
		base.Logging.StoreBodies = true
		base.LLMSync = LLMSyncConfig{Enabled: true, Environment: "production"}

	case "clustermode":
		base.Port = 6020
		base.Logging.StoreBodies = true
		base.ClusterMode = ClusterModeConfig{Enabled: true, NodeID: "node-1"}

	case "encryptvault":
		base.Port = 6030
		base.Logging.StoreBodies = true
		base.EncryptVault = EncryptVaultConfig{Enabled: true}

	case "mirrortest":
		base.Port = 6040
		base.Logging.StoreBodies = true
		base.MirrorTest = MirrorTestConfig{Enabled: true, SampleRate: 0.1}

	default:
		if !applyPhase4Defaults(product, base) {
			base.Port = 4200
		}
	}

	return base
}
