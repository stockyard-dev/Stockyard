package config

// Phase 4 config structs (57 products)

type ExtractMLConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type TableForgeConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type ToolRouterConfig struct {
	Enabled bool         `yaml:"enabled" json:"enabled"`
	Tools   []ToolRegDef `yaml:"tools" json:"tools"`
}
type ToolRegDef struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

type ToolShieldConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	BlockedTools  []string `yaml:"blocked_tools" json:"blocked_tools"`
}
type ToolMockConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type AuthGateConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Keys    []string `yaml:"keys" json:"keys"`
}

type ScopeGuardConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Roles   []ScopeRole   `yaml:"roles" json:"roles"`
}
type ScopeRole struct {
	Name          string   `yaml:"name" json:"name"`
	AllowedModels []string `yaml:"allowed_models" json:"allowed_models"`
}

type VisionProxyConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type AudioProxyConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type DocParseConfig struct {
	Enabled   bool `yaml:"enabled" json:"enabled"`
	ChunkSize int  `yaml:"chunk_size" json:"chunk_size"`
}
type FrameGrabConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type SessionStoreConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	MaxSessions int `yaml:"max_sessions" json:"max_sessions"`
}
type ConvoForkConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type SlotFillConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Slots   []SlotDef  `yaml:"slots" json:"slots"`
}
type SlotDef struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Required bool   `yaml:"required" json:"required"`
}

type SemanticCacheConfig struct {
	Enabled   bool    `yaml:"enabled" json:"enabled"`
	Threshold float64 `yaml:"threshold" json:"threshold"`
}
type PartialCacheConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type StreamCacheConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type PromptChainConfig struct {
	Enabled bool            `yaml:"enabled" json:"enabled"`
	Blocks  []PromptBlock   `yaml:"blocks" json:"blocks"`
}
type PromptBlock struct {
	Name    string `yaml:"name" json:"name"`
	Content string `yaml:"content" json:"content"`
}

type PromptFuzzConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type PromptMarketConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type CostPredictConfig struct {
	Enabled  bool    `yaml:"enabled" json:"enabled"`
	MaxCost  float64 `yaml:"max_cost" json:"max_cost"`
}
type CostMapConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type SpotPriceConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type LoadForgeConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type SnapshotTestConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type ChaosLLMConfig struct {
	Enabled   bool    `yaml:"enabled" json:"enabled"`
	ErrorRate float64 `yaml:"error_rate" json:"error_rate"`
}

type DataMapConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type ConsentGateConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type RetentionWipeConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	RetentionDays  int      `yaml:"retention_days" json:"retention_days"`
}
type PolicyEngineConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type StreamSplitConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type StreamThrottleConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	MaxTokensPerSec int `yaml:"max_tokens_per_sec" json:"max_tokens_per_sec"`
}
type StreamTransformConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type ModelAliasConfig struct {
	Enabled bool           `yaml:"enabled" json:"enabled"`
	Aliases []ModelAliasDef `yaml:"aliases" json:"aliases"`
}
type ModelAliasDef struct {
	Alias string `yaml:"alias" json:"alias"`
	Model string `yaml:"model" json:"model"`
}

type ParamNormConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type QuotaSyncConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type ErrorNormConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type CohortTrackConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type PromptRankConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type AnomalyRadarConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type EnvSyncConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type ProxyLogConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type CliDashConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type EmbedRouterConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type FineTuneTrackConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type AgentReplayConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type SummarizeGateConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type CodeLangConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }

type PersonaSwitchConfig struct {
	Enabled  bool         `yaml:"enabled" json:"enabled"`
	Personas []PersonaDef `yaml:"personas" json:"personas"`
}
type PersonaDef struct {
	Name         string  `yaml:"name" json:"name"`
	SystemPrompt string  `yaml:"system_prompt" json:"system_prompt"`
	Temperature  float64 `yaml:"temperature" json:"temperature"`
}

type WarmPoolConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type EdgeCacheConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type QueuePriorityConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type GeoPriceConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type TokenAuctionConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type CanaryDeployConfig struct {
	Enabled    bool    `yaml:"enabled" json:"enabled"`
	NewModel   string  `yaml:"new_model" json:"new_model"`
	TrafficPct float64 `yaml:"traffic_pct" json:"traffic_pct"`
}
type PlaybackStudioConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
type WebhookForgeConfig struct{ Enabled bool `yaml:"enabled" json:"enabled"` }
