// Stockyard — The operating system for AI software.
//
// Single binary shipping the complete platform:
//   - Proxy:        Core reverse-proxy, 66-module middleware chain, 16 providers
//   - 6 Core Apps:  Observe, Trust, Studio, Forge, Exchange, Billing
//   - 18 Products:  Bid, Replay, Doubt, Verdikt, Stampede, Fault, Morph, Spine,
//                   Hollow, Séance, Grain, Echo, Tide, Fossil, Prism, Trailhead,
//                   Iron, Orchestrator — tier-gated at Individual/Pro/Team/Enterprise
//
// One binary. One port. One deploy.
// Products activate based on license tier and can be toggled from the dashboard.
package main

import (
	"github.com/stockyard-dev/stockyard/internal/apps/appbuilder"
	"github.com/stockyard-dev/stockyard/internal/apps/billing"
	"github.com/stockyard-dev/stockyard/internal/apps/copilot"
	"github.com/stockyard-dev/stockyard/internal/apps/exchange"
	"github.com/stockyard-dev/stockyard/internal/apps/forge"
	"github.com/stockyard-dev/stockyard/internal/apps/governance"
	"github.com/stockyard-dev/stockyard/internal/apps/knowledge"
	"github.com/stockyard-dev/stockyard/internal/apps/marketing"
	"github.com/stockyard-dev/stockyard/internal/apps/memory"
	"github.com/stockyard-dev/stockyard/internal/apps/observe"
	proxyapp "github.com/stockyard-dev/stockyard/internal/apps/proxy"
	"github.com/stockyard-dev/stockyard/internal/apps/recall"
	"github.com/stockyard-dev/stockyard/internal/apps/reputation"
	"github.com/stockyard-dev/stockyard/internal/apps/studio"
	"github.com/stockyard-dev/stockyard/internal/apps/team"
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
		Name:            "Stockyard",
		Product:         "stockyard",
		Version:         version,
		EnableAPIServer: true,
		Apps: []platform.App{
			proxyapp.New(nil),
			observe.New(nil),
			trust.New(nil),
			studio.New(nil),
			forge.New(nil),
			exchange.New(nil),
			billing.New(nil),
			team.New(nil),
			memory.New(nil),
			marketing.New(nil),
			recall.New(nil),
			copilot.New(nil),
			appbuilder.New(nil),
			knowledge.New(nil),
			reputation.New(nil),
			governance.New(nil),
		},
		Features: engine.Features{
			SpendTracking:   true,
			SpendCaps:       true,
			Alerts:          true,
			Cache:           true,
			Validation:      true,
			Failover:        true,
			RateLimiting:    true,
			RequestLogging:  true,
			FullBodyLog:     true,
			KeyPool:         true,
			PromptGuard:     true,
			ModelSwitch:     true,
			EvalGate:        true,
			UsagePulse:      true,
			PromptPad:       true,
			TokenTrim:       true,
			BatchQueue:      true,
			MultiCall:       true,
			StreamSnap:      true,
			LLMTap:          true,
			ContextPack:     true,
			RetryPilot:      true,
			ToxicFilter:     true,
			ComplianceLog:   true,
			SecretScan:      true,
			TraceLink:       true,
			IPFence:         true,
			EmbedCache:      true,
			AnthroFit:       true,
			AlertPulse:      true,
			ChatMem:         true,
			MockLLM:         true,
			TenantWall:      true,
			IdleKill:        true,
			AgentGuard:      true,
			CodeFence:       true,
			HalluciCheck:    true,
			TierDrop:        true,
			DriftWatch:      true,
			FeedbackLoop:    true,
			ABRouter:        true,
			GuardRail:       true,
			GeminiShim:      true,
			LocalSync:       true,
			DevProxy:        true,
			PromptSlim:      true,
			PromptLint:      true,
			ApprovalGate:    true,
			OutputCap:       true,
			AgeGate:         true,
			VoiceBridge:     true,
			ImageProxy:      true,
			LangBridge:      true,
			ContextWindow:   true,
			RegionRoute:     true,
			BillingMeter:    true,
			ChainForge:      true,
			CronLLM:         true,
			WebhookRelay:    true,
			BillSync:        true,
			WhiteLabel:      true,
			TrainExport:     true,
			SynthGen:        true,
			DiffPrompt:      true,
			LLMBench:        true,
			MaskMode:        true,
			TokenMarket:     true,
			LLMSync:         true,
			ClusterMode:     true,
			EncryptVault:    true,
			MirrorTest:      true,
			ExtractML:       true,
			TableForge:      true,
			ToolRouter:      true,
			ToolShield:      true,
			ToolMock:        true,
			AuthGate:        true,
			ScopeGuard:      true,
			VisionProxy:     true,
			AudioProxy:      true,
			DocParse:        true,
			FrameGrab:       true,
			SessionStore:    true,
			ConvoFork:       true,
			SlotFill:        true,
			SemanticCache:   true,
			PartialCache:    true,
			StreamCache:     true,
			PromptChain:     true,
			PromptFuzz:      true,
			PromptMarket:    true,
			CostPredict:     true,
			CostMap:         true,
			SpotPrice:       true,
			LoadForge:       true,
			SnapshotTest:    true,
			ChaosLLM:        true,
			DataMap:         true,
			ConsentGate:     true,
			RetentionWipe:   true,
			PolicyEngine:    true,
			StreamSplit:     true,
			StreamThrottle:  true,
			StreamTransform: true,
			ModelAlias:      true,
			ParamNorm:       true,
			QuotaSync:       true,
			ErrorNorm:       true,
			CohortTrack:     true,
			PromptRank:      true,
			AnomalyRadar:    true,
			EnvSync:         true,
			ProxyLog:        true,
			CliDash:         true,
			EmbedRouter:     true,
			FineTuneTrack:   true,
			AgentReplay:     true,
			SummarizeGate:   true,
			CodeLang:        true,
			PersonaSwitch:   true,
			WarmPool:        true,
			EdgeCache:       true,
			QueuePriority:   true,
			GeoPrice:        true,
			TokenAuction:    true,
			CanaryDeploy:    true,
			PlaybackStudio:  true,
			WebhookForge:    true,
			Consensus:       true,
			AutoTag:         true,
			Ghost:           true,
			PromptCompress:  true,
			Honeypot:        true,
			MeshRoute:       true,
		},
	})
}
