// Stockyard — "Where LLM traffic gets sorted."
//
// Single binary shipping all 6 flagship apps:
//   - Proxy:    Core reverse-proxy, middleware chain, provider dispatch
//   - Observe:  Analytics, traces, alerts, anomaly detection, cost attribution
//   - Trust:    Audit ledger, compliance, evidence packs, replay lab
//   - Studio:   Prompt templates, experiments, benchmarks, snapshot tests
//   - Forge:    Workflow engine, tool registry, triggers, sessions, batch
//   - Exchange: Pack marketplace, config sharing, environment sync
package main

import (
	"github.com/stockyard-dev/stockyard/internal/apps/exchange"
	"github.com/stockyard-dev/stockyard/internal/apps/forge"
	"github.com/stockyard-dev/stockyard/internal/apps/observe"
	proxyapp "github.com/stockyard-dev/stockyard/internal/apps/proxy"
	"github.com/stockyard-dev/stockyard/internal/apps/studio"
	"github.com/stockyard-dev/stockyard/internal/apps/trust"
	"github.com/stockyard-dev/stockyard/internal/engine"
	"github.com/stockyard-dev/stockyard/internal/platform"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	engine.Boot(engine.ProductConfig{
		Name:    "Stockyard",
		Product: "stockyard",
		Version: version,
		Apps: []platform.App{
			proxyapp.New(nil),
			observe.New(nil),
			trust.New(nil),
			studio.New(nil),
			forge.New(nil),
			exchange.New(nil),
		},
		Features: engine.Features{
			SpendTracking:  true,
			SpendCaps:      true,
			Alerts:         true,
			Cache:          true,
			Validation:     true,
			Failover:       true,
			RateLimiting:   true,
			RequestLogging: true,
			FullBodyLog:    true,
			KeyPool:     true,
			PromptGuard: true,
			ModelSwitch: true,
			EvalGate:    true,
			UsagePulse:  true,
			PromptPad:   true,
			TokenTrim:   true,
			BatchQueue:  true,
			MultiCall:   true,
			StreamSnap:  true,
			LLMTap:      true,
			ContextPack: true,
			RetryPilot:  true,
			ToxicFilter:   true,
			ComplianceLog: true,
			SecretScan:    true,
			TraceLink:     true,
			IPFence:       true,
			EmbedCache:    true,
			AnthroFit:     true,
			AlertPulse:    true,
			ChatMem:       true,
			MockLLM:       true,
			TenantWall:    true,
			IdleKill:      true,
			AgentGuard:    true,
			CodeFence:     true,
			HalluciCheck:  true,
			TierDrop:      true,
			DriftWatch:    true,
			FeedbackLoop:  true,
			ABRouter:      true,
			GuardRail:     true,
			GeminiShim:    true,
			LocalSync:     true,
			DevProxy:      true,
			PromptSlim:    true,
			PromptLint:    true,
			ApprovalGate:  true,
			OutputCap:     true,
			AgeGate:       true,
			VoiceBridge:   true,
			ImageProxy:    true,
			LangBridge:    true,
			ContextWindow: true,
			RegionRoute:   true,
			ChainForge:    true,
			CronLLM:       true,
			WebhookRelay:  true,
			BillSync:      true,
			WhiteLabel:    true,
			TrainExport:   true,
			SynthGen:      true,
			DiffPrompt:    true,
			LLMBench:      true,
			MaskMode:      true,
			TokenMarket:   true,
			LLMSync:       true,
			ClusterMode:   true,
			EncryptVault:  true,
			MirrorTest:    true,
			ExtractML:      true,
			TableForge:     true,
			ToolRouter:     true,
			ToolShield:     true,
			ToolMock:       true,
			AuthGate:       true,
			ScopeGuard:     true,
			VisionProxy:    true,
			AudioProxy:     true,
			DocParse:       true,
			FrameGrab:      true,
			SessionStore:   true,
			ConvoFork:      true,
			SlotFill:       true,
			SemanticCache:  true,
			PartialCache:   true,
			StreamCache:    true,
			PromptChain:    true,
			PromptFuzz:     true,
			PromptMarket:   true,
			CostPredict:    true,
			CostMap:        true,
			SpotPrice:      true,
			LoadForge:      true,
			SnapshotTest:   true,
			ChaosLLM:       true,
			DataMap:        true,
			ConsentGate:    true,
			RetentionWipe:  true,
			PolicyEngine:   true,
			StreamSplit:    true,
			StreamThrottle: true,
			StreamTransform:true,
			ModelAlias:     true,
			ParamNorm:      true,
			QuotaSync:      true,
			ErrorNorm:      true,
			CohortTrack:    true,
			PromptRank:     true,
			AnomalyRadar:   true,
			EnvSync:        true,
			ProxyLog:       true,
			CliDash:        true,
			EmbedRouter:    true,
			FineTuneTrack:  true,
			AgentReplay:    true,
			SummarizeGate:  true,
			CodeLang:       true,
			PersonaSwitch:  true,
			WarmPool:       true,
			EdgeCache:      true,
			QueuePriority:  true,
			GeoPrice:       true,
			TokenAuction:   true,
			CanaryDeploy:   true,
			PlaybackStudio: true,
			WebhookForge:   true,
		},
	})
}
