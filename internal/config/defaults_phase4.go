package config

// Phase 4 product defaults — called from ProductDefaults
func applyPhase4Defaults(product string, base *Config) bool {
	switch product {
	case "extractml":
		base.Port = 6050; base.Logging.StoreBodies = true
		base.ExtractML = ExtractMLConfig{Enabled: true}
	case "tableforge":
		base.Port = 6060; base.Logging.StoreBodies = true
		base.TableForge = TableForgeConfig{Enabled: true}
	case "toolrouter":
		base.Port = 6070; base.Logging.StoreBodies = true
		base.ToolRouter = ToolRouterConfig{Enabled: true}
	case "toolshield":
		base.Port = 6080; base.Logging.StoreBodies = true
		base.ToolShield = ToolShieldConfig{Enabled: true}
	case "toolmock":
		base.Port = 6090; base.Logging.StoreBodies = true
		base.ToolMock = ToolMockConfig{Enabled: true}
	case "authgate":
		base.Port = 6100; base.Logging.StoreBodies = true
		base.AuthGate = AuthGateConfig{Enabled: true}
	case "scopeguard":
		base.Port = 6110; base.Logging.StoreBodies = true
		base.ScopeGuard = ScopeGuardConfig{Enabled: true}
	case "visionproxy":
		base.Port = 6120; base.Logging.StoreBodies = true
		base.VisionProxy = VisionProxyConfig{Enabled: true}
	case "audioproxy":
		base.Port = 6130; base.Logging.StoreBodies = true
		base.AudioProxy = AudioProxyConfig{Enabled: true}
	case "docparse":
		base.Port = 6140; base.Logging.StoreBodies = true
		base.DocParse = DocParseConfig{Enabled: true, ChunkSize: 512}
	case "framegrab":
		base.Port = 6150; base.Logging.StoreBodies = true
		base.FrameGrab = FrameGrabConfig{Enabled: true}
	case "sessionstore":
		base.Port = 6160; base.Logging.StoreBodies = true
		base.SessionStore = SessionStoreConfig{Enabled: true, MaxSessions: 10000}
	case "convofork":
		base.Port = 6170; base.Logging.StoreBodies = true
		base.ConvoFork = ConvoForkConfig{Enabled: true}
	case "slotfill":
		base.Port = 6180; base.Logging.StoreBodies = true
		base.SlotFill = SlotFillConfig{Enabled: true}
	case "semanticcache":
		base.Port = 6190; base.Logging.StoreBodies = true
		base.SemanticCache = SemanticCacheConfig{Enabled: true, Threshold: 0.92}
	case "partialcache":
		base.Port = 6200; base.Logging.StoreBodies = true
		base.PartialCache = PartialCacheConfig{Enabled: true}
	case "streamcache":
		base.Port = 6210; base.Logging.StoreBodies = true
		base.StreamCache = StreamCacheConfig{Enabled: true}
	case "promptchain":
		base.Port = 6220; base.Logging.StoreBodies = true
		base.PromptChain = PromptChainConfig{Enabled: true}
	case "promptfuzz":
		base.Port = 6230; base.Logging.StoreBodies = true
		base.PromptFuzz = PromptFuzzConfig{Enabled: true}
	case "promptmarket":
		base.Port = 6240; base.Logging.StoreBodies = true
		base.PromptMarket = PromptMarketConfig{Enabled: true}
	case "costpredict":
		base.Port = 6250; base.Logging.StoreBodies = true
		base.CostPredict = CostPredictConfig{Enabled: true}
	case "costmap":
		base.Port = 6260; base.Logging.StoreBodies = true
		base.CostMap = CostMapConfig{Enabled: true}
	case "spotprice":
		base.Port = 6270; base.Logging.StoreBodies = true
		base.SpotPrice = SpotPriceConfig{Enabled: true}
	case "loadforge":
		base.Port = 6280; base.Logging.StoreBodies = true
		base.LoadForge = LoadForgeConfig{Enabled: true}
	case "snapshottest":
		base.Port = 6290; base.Logging.StoreBodies = true
		base.SnapshotTest = SnapshotTestConfig{Enabled: true}
	case "chaosllm":
		base.Port = 6300; base.Logging.StoreBodies = true
		base.ChaosLLM = ChaosLLMConfig{Enabled: true, ErrorRate: 0.1}
	case "datamap":
		base.Port = 6310; base.Logging.StoreBodies = true
		base.DataMap = DataMapConfig{Enabled: true}
	case "consentgate":
		base.Port = 6320; base.Logging.StoreBodies = true
		base.ConsentGate = ConsentGateConfig{Enabled: true}
	case "retentionwipe":
		base.Port = 6330; base.Logging.StoreBodies = true
		base.RetentionWipe = RetentionWipeConfig{Enabled: true, RetentionDays: 90}
	case "policyengine":
		base.Port = 6340; base.Logging.StoreBodies = true
		base.PolicyEngine = PolicyEngineConfig{Enabled: true}
	case "streamsplit":
		base.Port = 6350; base.Logging.StoreBodies = true
		base.StreamSplit = StreamSplitConfig{Enabled: true}
	case "streamthrottle":
		base.Port = 6360; base.Logging.StoreBodies = true
		base.StreamThrottle = StreamThrottleConfig{Enabled: true, MaxTokensPerSec: 50}
	case "streamtransform":
		base.Port = 6370; base.Logging.StoreBodies = true
		base.StreamTransform = StreamTransformConfig{Enabled: true}
	case "modelalias":
		base.Port = 6380; base.Logging.StoreBodies = true
		base.ModelAlias = ModelAliasConfig{Enabled: true}
	case "paramnorm":
		base.Port = 6390; base.Logging.StoreBodies = true
		base.ParamNorm = ParamNormConfig{Enabled: true}
	case "quotasync":
		base.Port = 6400; base.Logging.StoreBodies = true
		base.QuotaSync = QuotaSyncConfig{Enabled: true}
	case "errornorm":
		base.Port = 6410; base.Logging.StoreBodies = true
		base.ErrorNorm = ErrorNormConfig{Enabled: true}
	case "cohorttrack":
		base.Port = 6420; base.Logging.StoreBodies = true
		base.CohortTrack = CohortTrackConfig{Enabled: true}
	case "promptrank":
		base.Port = 6430; base.Logging.StoreBodies = true
		base.PromptRank = PromptRankConfig{Enabled: true}
	case "anomalyradar":
		base.Port = 6440; base.Logging.StoreBodies = true
		base.AnomalyRadar = AnomalyRadarConfig{Enabled: true}
	case "envsync":
		base.Port = 6450; base.Logging.StoreBodies = true
		base.EnvSync = EnvSyncConfig{Enabled: true}
	case "proxylog":
		base.Port = 6460; base.Logging.StoreBodies = true
		base.ProxyLog = ProxyLogConfig{Enabled: true}
	case "clidash":
		base.Port = 6470; base.Logging.StoreBodies = true
		base.CliDash = CliDashConfig{Enabled: true}
	case "embedrouter":
		base.Port = 6480; base.Logging.StoreBodies = true
		base.EmbedRouter = EmbedRouterConfig{Enabled: true}
	case "finetunetrack":
		base.Port = 6490; base.Logging.StoreBodies = true
		base.FineTuneTrack = FineTuneTrackConfig{Enabled: true}
	case "agentreplay":
		base.Port = 6500; base.Logging.StoreBodies = true
		base.AgentReplay = AgentReplayConfig{Enabled: true}
	case "summarizegate":
		base.Port = 6510; base.Logging.StoreBodies = true
		base.SummarizeGate = SummarizeGateConfig{Enabled: true}
	case "codelang":
		base.Port = 6520; base.Logging.StoreBodies = true
		base.CodeLang = CodeLangConfig{Enabled: true}
	case "personaswitch":
		base.Port = 6530; base.Logging.StoreBodies = true
		base.PersonaSwitch = PersonaSwitchConfig{Enabled: true}
	case "warmpool":
		base.Port = 6540; base.Logging.StoreBodies = true
		base.WarmPool = WarmPoolConfig{Enabled: true}
	case "edgecache":
		base.Port = 6550; base.Logging.StoreBodies = true
		base.EdgeCache = EdgeCacheConfig{Enabled: true}
	case "queuepriority":
		base.Port = 6560; base.Logging.StoreBodies = true
		base.QueuePriority = QueuePriorityConfig{Enabled: true}
	case "geoprice":
		base.Port = 6570; base.Logging.StoreBodies = true
		base.GeoPrice = GeoPriceConfig{Enabled: true}
	case "tokenauction":
		base.Port = 6580; base.Logging.StoreBodies = true
		base.TokenAuction = TokenAuctionConfig{Enabled: true}
	case "canarydeploy":
		base.Port = 6590; base.Logging.StoreBodies = true
		base.CanaryDeploy = CanaryDeployConfig{Enabled: true, TrafficPct: 5.0}
	case "playbackstudio":
		base.Port = 6600; base.Logging.StoreBodies = true
		base.PlaybackStudio = PlaybackStudioConfig{Enabled: true}
	case "webhookforge":
		base.Port = 6610; base.Logging.StoreBodies = true
		base.WebhookForge = WebhookForgeConfig{Enabled: true}
	default:
		return false
	}
	return true
}
