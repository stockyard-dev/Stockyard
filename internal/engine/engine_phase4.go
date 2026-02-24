package engine

import (
	"log"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/features"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// buildPhase4Middlewares appends Phase 4 middleware to the chain.
func buildPhase4Middlewares(
	pc ProductConfig,
	cfg *config.Config,
	providers map[string]provider.Provider,
	mw []proxy.Middleware,
) []proxy.Middleware {

	// Auth & Access Control (early in chain)
	if pc.Features.AuthGate && cfg.AuthGate.Enabled {
		s := features.NewAuthGate(cfg.AuthGate); mw = append(mw, features.AuthGateMiddleware(s))
		log.Printf("authgate: %d keys", len(cfg.AuthGate.Keys))
	}
	if pc.Features.ScopeGuard && cfg.ScopeGuard.Enabled {
		s := features.NewScopeGuard(cfg.ScopeGuard); mw = append(mw, features.ScopeGuardMiddleware(s))
		log.Printf("scopeguard: %d roles", len(cfg.ScopeGuard.Roles))
	}
	if pc.Features.ConsentGate && cfg.ConsentGate.Enabled {
		s := features.NewConsentGate(cfg.ConsentGate); mw = append(mw, features.ConsentGateMiddleware(s))
		log.Printf("consentgate: enabled")
	}

	// Prompt transformation (before send)
	if pc.Features.PromptChain && cfg.PromptChain.Enabled {
		s := features.NewPromptChain(cfg.PromptChain); mw = append(mw, features.PromptChainMiddleware(s))
		log.Printf("promptchain: %d blocks", len(cfg.PromptChain.Blocks))
	}
	if pc.Features.PersonaSwitch && cfg.PersonaSwitch.Enabled {
		s := features.NewPersonaSwitch(cfg.PersonaSwitch); mw = append(mw, features.PersonaSwitchMiddleware(s))
		log.Printf("personaswitch: %d personas", len(cfg.PersonaSwitch.Personas))
	}
	if pc.Features.SummarizeGate && cfg.SummarizeGate.Enabled {
		s := features.NewSummarizeGate(cfg.SummarizeGate); mw = append(mw, features.SummarizeGateMiddleware(s))
		log.Printf("summarizegate: enabled")
	}

	// Cost intelligence (before send)
	if pc.Features.CostPredict && cfg.CostPredict.Enabled {
		s := features.NewCostPredict(cfg.CostPredict); mw = append(mw, features.CostPredictMiddleware(s))
		log.Printf("costpredict: enabled")
	}
	if pc.Features.SpotPrice && cfg.SpotPrice.Enabled {
		s := features.NewSpotPrice(cfg.SpotPrice); mw = append(mw, features.SpotPriceMiddleware(s))
		log.Printf("spotprice: enabled")
	}

	// Caching (before send)
	if pc.Features.SemanticCache && cfg.SemanticCache.Enabled {
		s := features.NewSemanticCacheFromConfig(cfg.SemanticCache); mw = append(mw, features.SemanticCacheMiddleware(s))
		log.Printf("semanticcache: threshold=%.2f", cfg.SemanticCache.Threshold)
	}
	if pc.Features.PartialCache && cfg.PartialCache.Enabled {
		s := features.NewPartialCache(cfg.PartialCache); mw = append(mw, features.PartialCacheMiddleware(s))
		log.Printf("partialcache: enabled")
	}
	if pc.Features.StreamCache && cfg.StreamCache.Enabled {
		s := features.NewStreamCache(cfg.StreamCache); mw = append(mw, features.StreamCacheMiddleware(s))
		log.Printf("streamcache: enabled")
	}

	// Routing (before send)
	if pc.Features.ModelAlias && cfg.ModelAlias.Enabled {
		s := features.NewModelAlias(cfg.ModelAlias); mw = append(mw, features.ModelAliasMiddleware(s))
		log.Printf("modelalias: %d aliases", len(cfg.ModelAlias.Aliases))
	}
	if pc.Features.CanaryDeploy && cfg.CanaryDeploy.Enabled {
		s := features.NewCanaryDeploy(cfg.CanaryDeploy); mw = append(mw, features.CanaryDeployMiddleware(s))
		log.Printf("canarydeploy: new=%s traffic=%.0f%%", cfg.CanaryDeploy.NewModel, cfg.CanaryDeploy.TrafficPct)
	}
	if pc.Features.ParamNorm && cfg.ParamNorm.Enabled {
		s := features.NewParamNorm(cfg.ParamNorm); mw = append(mw, features.ParamNormMiddleware(s))
		log.Printf("paramnorm: enabled")
	}
	if pc.Features.QuotaSync && cfg.QuotaSync.Enabled {
		s := features.NewQuotaSync(cfg.QuotaSync); mw = append(mw, features.QuotaSyncMiddleware(s))
		log.Printf("quotasync: enabled")
	}
	if pc.Features.ErrorNorm && cfg.ErrorNorm.Enabled {
		s := features.NewErrorNorm(cfg.ErrorNorm); mw = append(mw, features.ErrorNormMiddleware(s))
		log.Printf("errornorm: enabled")
	}

	// Streaming middleware
	if pc.Features.StreamThrottle && cfg.StreamThrottle.Enabled {
		s := features.NewStreamThrottle(cfg.StreamThrottle); mw = append(mw, features.StreamThrottleMiddleware(s))
		log.Printf("streamthrottle: max=%d tok/s", cfg.StreamThrottle.MaxTokensPerSec)
	}
	if pc.Features.StreamTransform && cfg.StreamTransform.Enabled {
		s := features.NewStreamTransform(cfg.StreamTransform); mw = append(mw, features.StreamTransformMiddleware(s))
		log.Printf("streamtransform: enabled")
	}
	if pc.Features.StreamSplit && cfg.StreamSplit.Enabled {
		s := features.NewStreamSplit(cfg.StreamSplit); mw = append(mw, features.StreamSplitMiddleware(s))
		log.Printf("streamsplit: enabled")
	}

	// Post-response validation
	if pc.Features.ExtractML && cfg.ExtractML.Enabled {
		s := features.NewExtractML(cfg.ExtractML); mw = append(mw, features.ExtractMLMiddleware(s))
		log.Printf("extractml: enabled")
	}
	if pc.Features.TableForge && cfg.TableForge.Enabled {
		s := features.NewTableForge(cfg.TableForge); mw = append(mw, features.TableForgeMiddleware(s))
		log.Printf("tableforge: enabled")
	}
	if pc.Features.CodeLang && cfg.CodeLang.Enabled {
		s := features.NewCodeLang(cfg.CodeLang); mw = append(mw, features.CodeLangMiddleware(s))
		log.Printf("codelang: enabled")
	}

	// Tool use
	if pc.Features.ToolRouter && cfg.ToolRouter.Enabled {
		s := features.NewToolRouter(cfg.ToolRouter); mw = append(mw, features.ToolRouterMiddleware(s))
		log.Printf("toolrouter: %d tools", len(cfg.ToolRouter.Tools))
	}
	if pc.Features.ToolShield && cfg.ToolShield.Enabled {
		s := features.NewToolShield(cfg.ToolShield); mw = append(mw, features.ToolShieldMiddleware(s))
		log.Printf("toolshield: enabled")
	}
	if pc.Features.ToolMock && cfg.ToolMock.Enabled {
		s := features.NewToolMock(cfg.ToolMock); mw = append(mw, features.ToolMockMiddleware(s))
		log.Printf("toolmock: enabled")
	}

	// Multimodal
	if pc.Features.VisionProxy && cfg.VisionProxy.Enabled {
		s := features.NewVisionProxy(cfg.VisionProxy); mw = append(mw, features.VisionProxyMiddleware(s))
		log.Printf("visionproxy: enabled")
	}
	if pc.Features.AudioProxy && cfg.AudioProxy.Enabled {
		s := features.NewAudioProxy(cfg.AudioProxy); mw = append(mw, features.AudioProxyMiddleware(s))
		log.Printf("audioproxy: enabled")
	}
	if pc.Features.DocParse && cfg.DocParse.Enabled {
		s := features.NewDocParse(cfg.DocParse); mw = append(mw, features.DocParseMiddleware(s))
		log.Printf("docparse: chunk_size=%d", cfg.DocParse.ChunkSize)
	}
	if pc.Features.FrameGrab && cfg.FrameGrab.Enabled {
		s := features.NewFrameGrab(cfg.FrameGrab); mw = append(mw, features.FrameGrabMiddleware(s))
		log.Printf("framegrab: enabled")
	}

	// Sessions
	if pc.Features.SessionStore && cfg.SessionStore.Enabled {
		s := features.NewSessionStore(cfg.SessionStore); mw = append(mw, features.SessionStoreMiddleware(s))
		log.Printf("sessionstore: max=%d", cfg.SessionStore.MaxSessions)
	}
	if pc.Features.ConvoFork && cfg.ConvoFork.Enabled {
		s := features.NewConvoFork(cfg.ConvoFork); mw = append(mw, features.ConvoForkMiddleware(s))
		log.Printf("convofork: enabled")
	}
	if pc.Features.SlotFill && cfg.SlotFill.Enabled {
		s := features.NewSlotFill(cfg.SlotFill); mw = append(mw, features.SlotFillMiddleware(s))
		log.Printf("slotfill: %d slots", len(cfg.SlotFill.Slots))
	}

	// Prompt management
	if pc.Features.PromptFuzz && cfg.PromptFuzz.Enabled {
		s := features.NewPromptFuzz(cfg.PromptFuzz); mw = append(mw, features.PromptFuzzMiddleware(s))
		log.Printf("promptfuzz: enabled")
	}
	if pc.Features.PromptMarket && cfg.PromptMarket.Enabled {
		s := features.NewPromptMarket(cfg.PromptMarket); mw = append(mw, features.PromptMarketMiddleware(s))
		log.Printf("promptmarket: enabled")
	}

	// Testing & QA
	if pc.Features.LoadForge && cfg.LoadForge.Enabled {
		s := features.NewLoadForge(cfg.LoadForge); mw = append(mw, features.LoadForgeMiddleware(s))
		log.Printf("loadforge: enabled")
	}
	if pc.Features.SnapshotTest && cfg.SnapshotTest.Enabled {
		s := features.NewSnapshotTest(cfg.SnapshotTest); mw = append(mw, features.SnapshotTestMiddleware(s))
		log.Printf("snapshottest: enabled")
	}
	if pc.Features.ChaosLLM && cfg.ChaosLLM.Enabled {
		s := features.NewChaosLLM(cfg.ChaosLLM); mw = append(mw, features.ChaosLLMMiddleware(s))
		log.Printf("chaosllm: error_rate=%.0f%%", cfg.ChaosLLM.ErrorRate*100)
	}

	// Compliance
	if pc.Features.DataMap && cfg.DataMap.Enabled {
		s := features.NewDataMap(cfg.DataMap); mw = append(mw, features.DataMapMiddleware(s))
		log.Printf("datamap: enabled")
	}
	if pc.Features.RetentionWipe && cfg.RetentionWipe.Enabled {
		s := features.NewRetentionWipe(cfg.RetentionWipe); mw = append(mw, features.RetentionWipeMiddleware(s))
		log.Printf("retentionwipe: %d days", cfg.RetentionWipe.RetentionDays)
	}
	if pc.Features.PolicyEngine && cfg.PolicyEngine.Enabled {
		s := features.NewPolicyEngine(cfg.PolicyEngine); mw = append(mw, features.PolicyEngineMiddleware(s))
		log.Printf("policyengine: enabled")
	}

	// Analytics & Observability
	if pc.Features.CostMap && cfg.CostMap.Enabled {
		s := features.NewCostMap(cfg.CostMap); mw = append(mw, features.CostMapMiddleware(s))
		log.Printf("costmap: enabled")
	}
	if pc.Features.CohortTrack && cfg.CohortTrack.Enabled {
		s := features.NewCohortTrack(cfg.CohortTrack); mw = append(mw, features.CohortTrackMiddleware(s))
		log.Printf("cohorttrack: enabled")
	}
	if pc.Features.PromptRank && cfg.PromptRank.Enabled {
		s := features.NewPromptRank(cfg.PromptRank); mw = append(mw, features.PromptRankMiddleware(s))
		log.Printf("promptrank: enabled")
	}
	if pc.Features.AnomalyRadar && cfg.AnomalyRadar.Enabled {
		s := features.NewAnomalyRadar(cfg.AnomalyRadar); mw = append(mw, features.AnomalyRadarMiddleware(s))
		log.Printf("anomalyradar: enabled")
	}
	if pc.Features.ProxyLog && cfg.ProxyLog.Enabled {
		s := features.NewProxyLog(cfg.ProxyLog); mw = append(mw, features.ProxyLogMiddleware(s))
		log.Printf("proxylog: enabled")
	}

	// Dev Workflow
	if pc.Features.EnvSync && cfg.EnvSync.Enabled {
		s := features.NewEnvSync(cfg.EnvSync); mw = append(mw, features.EnvSyncMiddleware(s))
		log.Printf("envsync: enabled")
	}
	if pc.Features.CliDash && cfg.CliDash.Enabled {
		s := features.NewCliDash(cfg.CliDash); mw = append(mw, features.CliDashMiddleware(s))
		log.Printf("clidash: enabled")
	}

	// Specialized
	if pc.Features.EmbedRouter && cfg.EmbedRouter.Enabled {
		s := features.NewEmbedRouter(cfg.EmbedRouter); mw = append(mw, features.EmbedRouterMiddleware(s))
		log.Printf("embedrouter: enabled")
	}
	if pc.Features.FineTuneTrack && cfg.FineTuneTrack.Enabled {
		s := features.NewFineTuneTrack(cfg.FineTuneTrack); mw = append(mw, features.FineTuneTrackMiddleware(s))
		log.Printf("finetunetrack: enabled")
	}
	if pc.Features.AgentReplay && cfg.AgentReplay.Enabled {
		s := features.NewAgentReplay(cfg.AgentReplay); mw = append(mw, features.AgentReplayMiddleware(s))
		log.Printf("agentreplay: enabled")
	}

	// Infrastructure
	if pc.Features.WarmPool && cfg.WarmPool.Enabled {
		s := features.NewWarmPool(cfg.WarmPool); mw = append(mw, features.WarmPoolMiddleware(s))
		log.Printf("warmpool: enabled")
	}
	if pc.Features.EdgeCache && cfg.EdgeCache.Enabled {
		s := features.NewEdgeCache(cfg.EdgeCache); mw = append(mw, features.EdgeCacheMiddleware(s))
		log.Printf("edgecache: enabled")
	}
	if pc.Features.QueuePriority && cfg.QueuePriority.Enabled {
		s := features.NewQueuePriority(cfg.QueuePriority); mw = append(mw, features.QueuePriorityMiddleware(s))
		log.Printf("queuepriority: enabled")
	}
	if pc.Features.GeoPrice && cfg.GeoPrice.Enabled {
		s := features.NewGeoPrice(cfg.GeoPrice); mw = append(mw, features.GeoPriceMiddleware(s))
		log.Printf("geoprice: enabled")
	}

	// Niche
	if pc.Features.TokenAuction && cfg.TokenAuction.Enabled {
		s := features.NewTokenAuction(cfg.TokenAuction); mw = append(mw, features.TokenAuctionMiddleware(s))
		log.Printf("tokenauction: enabled")
	}
	if pc.Features.PlaybackStudio && cfg.PlaybackStudio.Enabled {
		s := features.NewPlaybackStudio(cfg.PlaybackStudio); mw = append(mw, features.PlaybackStudioMiddleware(s))
		log.Printf("playbackstudio: enabled")
	}
	if pc.Features.WebhookForge && cfg.WebhookForge.Enabled {
		s := features.NewWebhookForge(cfg.WebhookForge); mw = append(mw, features.WebhookForgeMiddleware(s))
		log.Printf("webhookforge: enabled")
	}

	return mw
}
