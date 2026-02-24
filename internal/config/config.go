// Package config handles YAML configuration parsing with environment variable interpolation.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for an Stockyard product.
type Config struct {
	Port     int    `yaml:"port" json:"port"`
	DataDir  string `yaml:"data_dir" json:"data_dir"`
	LogLevel string `yaml:"log_level" json:"log_level"`
	Product  string `yaml:"product" json:"product"`

	Providers map[string]ProviderConfig `yaml:"providers" json:"providers"`
	Projects  map[string]ProjectConfig  `yaml:"projects" json:"projects"`

	Cache      CacheConfig      `yaml:"cache" json:"cache"`
	Validation ValidationConfig `yaml:"validation" json:"validation"`
	Failover   FailoverConfig   `yaml:"failover" json:"failover"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit" json:"rate_limit"`
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`

	// Phase 1 expansion features
	KeyPool     KeyPoolConfig     `yaml:"keypool" json:"keypool"`
	PromptGuard PromptGuardConfig `yaml:"promptguard" json:"promptguard"`
	ModelSwitch ModelSwitchConfig `yaml:"modelswitch" json:"modelswitch"`
	EvalGate    EvalGateConfig    `yaml:"evalgate" json:"evalgate"`
	UsagePulse  UsagePulseConfig  `yaml:"usagepulse" json:"usagepulse"`

	// Phase 2 expansion features
	PromptPad   PromptPadConfig   `yaml:"promptpad" json:"promptpad"`
	TokenTrim   TokenTrimConfig   `yaml:"tokentrim" json:"tokentrim"`
	BatchQueue  BatchQueueConfig  `yaml:"batchqueue" json:"batchqueue"`
	MultiCall   MultiCallConfig   `yaml:"multicall" json:"multicall"`
	StreamSnap  StreamSnapConfig  `yaml:"streamsnap" json:"streamsnap"`
	LLMTap      LLMTapConfig      `yaml:"llmtap" json:"llmtap"`
	ContextPack ContextPackConfig `yaml:"contextpack" json:"contextpack"`
	RetryPilot  RetryPilotConfig  `yaml:"retrypilot" json:"retrypilot"`

	// Phase 3 expansion features
	ToxicFilter   ToxicFilterConfig   `yaml:"toxicfilter" json:"toxicfilter"`
	ComplianceLog ComplianceLogConfig `yaml:"compliancelog" json:"compliancelog"`
	SecretScan    SecretScanConfig    `yaml:"secretscan" json:"secretscan"`
	TraceLink     TraceLinkConfig     `yaml:"tracelink" json:"tracelink"`
	IPFence       IPFenceConfig       `yaml:"ipfence" json:"ipfence"`
	EmbedCache    EmbedCacheConfig    `yaml:"embedcache" json:"embedcache"`
	AnthroFit     AnthroFitConfig     `yaml:"anthrofit" json:"anthrofit"`
	AlertPulse    AlertPulseConfig    `yaml:"alertpulse" json:"alertpulse"`
	ChatMem       ChatMemConfig       `yaml:"chatmem" json:"chatmem"`
	MockLLM       MockLLMConfig       `yaml:"mockllm" json:"mockllm"`
	TenantWall    TenantWallConfig    `yaml:"tenantwall" json:"tenantwall"`
	IdleKill      IdleKillConfig      `yaml:"idlekill" json:"idlekill"`

	// Phase 3 P2 expansion features
	AgentGuard    AgentGuardConfig    `yaml:"agentguard" json:"agentguard"`
	CodeFence     CodeFenceConfig     `yaml:"codefence" json:"codefence"`
	HalluciCheck  HalluciCheckConfig  `yaml:"hallucicheck" json:"hallucicheck"`
	TierDrop      TierDropConfig      `yaml:"tierdrop" json:"tierdrop"`
	DriftWatch    DriftWatchConfig    `yaml:"driftwatch" json:"driftwatch"`
	FeedbackLoop  FeedbackLoopConfig  `yaml:"feedbackloop" json:"feedbackloop"`
	ABRouter      ABRouterConfig      `yaml:"abrouter" json:"abrouter"`
	GuardRail     GuardRailConfig     `yaml:"guardrail" json:"guardrail"`
	GeminiShim    GeminiShimConfig    `yaml:"geminishim" json:"geminishim"`
	LocalSync     LocalSyncConfig     `yaml:"localsync" json:"localsync"`
	DevProxy      DevProxyConfig      `yaml:"devproxy" json:"devproxy"`
	PromptSlim    PromptSlimConfig    `yaml:"promptslim" json:"promptslim"`
	PromptLint    PromptLintConfig    `yaml:"promptlint" json:"promptlint"`
	ApprovalGate  ApprovalGateConfig  `yaml:"approvalgate" json:"approvalgate"`
	OutputCap     OutputCapConfig     `yaml:"outputcap" json:"outputcap"`
	AgeGate       AgeGateConfig       `yaml:"agegate" json:"agegate"`
	VoiceBridge   VoiceBridgeConfig   `yaml:"voicebridge" json:"voicebridge"`
	ImageProxy    ImageProxyConfig    `yaml:"imageproxy" json:"imageproxy"`
	LangBridge    LangBridgeConfig    `yaml:"langbridge" json:"langbridge"`
	ContextWindow ContextWindowConfig `yaml:"contextwindow" json:"contextwindow"`
	RegionRoute   RegionRouteConfig   `yaml:"regionroute" json:"regionroute"`

	// Phase 3 P3 expansion features
	ChainForge    ChainForgeConfig    `yaml:"chainforge" json:"chainforge"`
	CronLLM       CronLLMConfig       `yaml:"cronllm" json:"cronllm"`
	WebhookRelay  WebhookRelayConfig  `yaml:"webhookrelay" json:"webhookrelay"`
	BillSync      BillSyncConfig      `yaml:"billsync" json:"billsync"`
	WhiteLabel    WhiteLabelConfig    `yaml:"whitelabel" json:"whitelabel"`
	TrainExport   TrainExportConfig   `yaml:"trainexport" json:"trainexport"`
	SynthGen      SynthGenConfig      `yaml:"synthgen" json:"synthgen"`
	DiffPrompt    DiffPromptConfig    `yaml:"diffprompt" json:"diffprompt"`
	LLMBench      LLMBenchConfig      `yaml:"llmbench" json:"llmbench"`
	MaskMode      MaskModeConfig      `yaml:"maskmode" json:"maskmode"`
	TokenMarket   TokenMarketConfig   `yaml:"tokenmarket" json:"tokenmarket"`
	LLMSync       LLMSyncConfig       `yaml:"llmsync" json:"llmsync"`
	ClusterMode   ClusterModeConfig   `yaml:"clustermode" json:"clustermode"`
	EncryptVault  EncryptVaultConfig  `yaml:"encryptvault" json:"encryptvault"`
	MirrorTest    MirrorTestConfig    `yaml:"mirrortest" json:"mirrortest"`

	// Phase 4 expansion features
	ExtractML      ExtractMLConfig      `yaml:"extractml" json:"extractml"`
	TableForge     TableForgeConfig     `yaml:"tableforge" json:"tableforge"`
	ToolRouter     ToolRouterConfig     `yaml:"toolrouter" json:"toolrouter"`
	ToolShield     ToolShieldConfig     `yaml:"toolshield" json:"toolshield"`
	ToolMock       ToolMockConfig       `yaml:"toolmock" json:"toolmock"`
	AuthGate       AuthGateConfig       `yaml:"authgate" json:"authgate"`
	ScopeGuard     ScopeGuardConfig     `yaml:"scopeguard" json:"scopeguard"`
	VisionProxy    VisionProxyConfig    `yaml:"visionproxy" json:"visionproxy"`
	AudioProxy     AudioProxyConfig     `yaml:"audioproxy" json:"audioproxy"`
	DocParse       DocParseConfig       `yaml:"docparse" json:"docparse"`
	FrameGrab      FrameGrabConfig      `yaml:"framegrab" json:"framegrab"`
	SessionStore   SessionStoreConfig   `yaml:"sessionstore" json:"sessionstore"`
	ConvoFork      ConvoForkConfig      `yaml:"convofork" json:"convofork"`
	SlotFill       SlotFillConfig       `yaml:"slotfill" json:"slotfill"`
	SemanticCache  SemanticCacheConfig  `yaml:"semanticcache" json:"semanticcache"`
	PartialCache   PartialCacheConfig   `yaml:"partialcache" json:"partialcache"`
	StreamCache    StreamCacheConfig    `yaml:"streamcache" json:"streamcache"`
	PromptChain    PromptChainConfig    `yaml:"promptchain" json:"promptchain"`
	PromptFuzz     PromptFuzzConfig     `yaml:"promptfuzz" json:"promptfuzz"`
	PromptMarket   PromptMarketConfig   `yaml:"promptmarket" json:"promptmarket"`
	CostPredict    CostPredictConfig    `yaml:"costpredict" json:"costpredict"`
	CostMap        CostMapConfig        `yaml:"costmap" json:"costmap"`
	SpotPrice      SpotPriceConfig      `yaml:"spotprice" json:"spotprice"`
	LoadForge      LoadForgeConfig      `yaml:"loadforge" json:"loadforge"`
	SnapshotTest   SnapshotTestConfig   `yaml:"snapshottest" json:"snapshottest"`
	ChaosLLM       ChaosLLMConfig       `yaml:"chaosllm" json:"chaosllm"`
	DataMap        DataMapConfig        `yaml:"datamap" json:"datamap"`
	ConsentGate    ConsentGateConfig    `yaml:"consentgate" json:"consentgate"`
	RetentionWipe  RetentionWipeConfig  `yaml:"retentionwipe" json:"retentionwipe"`
	PolicyEngine   PolicyEngineConfig   `yaml:"policyengine" json:"policyengine"`
	StreamSplit    StreamSplitConfig    `yaml:"streamsplit" json:"streamsplit"`
	StreamThrottle StreamThrottleConfig `yaml:"streamthrottle" json:"streamthrottle"`
	StreamTransform StreamTransformConfig `yaml:"streamtransform" json:"streamtransform"`
	ModelAlias     ModelAliasConfig     `yaml:"modelalias" json:"modelalias"`
	ParamNorm      ParamNormConfig      `yaml:"paramnorm" json:"paramnorm"`
	QuotaSync      QuotaSyncConfig      `yaml:"quotasync" json:"quotasync"`
	ErrorNorm      ErrorNormConfig      `yaml:"errornorm" json:"errornorm"`
	CohortTrack    CohortTrackConfig    `yaml:"cohorttrack" json:"cohorttrack"`
	PromptRank     PromptRankConfig     `yaml:"promptrank" json:"promptrank"`
	AnomalyRadar   AnomalyRadarConfig   `yaml:"anomalyradar" json:"anomalyradar"`
	EnvSync        EnvSyncConfig        `yaml:"envsync" json:"envsync"`
	ProxyLog       ProxyLogConfig       `yaml:"proxylog" json:"proxylog"`
	CliDash        CliDashConfig        `yaml:"clidash" json:"clidash"`
	EmbedRouter    EmbedRouterConfig    `yaml:"embedrouter" json:"embedrouter"`
	FineTuneTrack  FineTuneTrackConfig  `yaml:"finetunetrack" json:"finetunetrack"`
	AgentReplay    AgentReplayConfig    `yaml:"agentreplay" json:"agentreplay"`
	SummarizeGate  SummarizeGateConfig  `yaml:"summarizegate" json:"summarizegate"`
	CodeLang       CodeLangConfig       `yaml:"codelang" json:"codelang"`
	PersonaSwitch  PersonaSwitchConfig  `yaml:"personaswitch" json:"personaswitch"`
	WarmPool       WarmPoolConfig       `yaml:"warmpool" json:"warmpool"`
	EdgeCache      EdgeCacheConfig      `yaml:"edgecache" json:"edgecache"`
	QueuePriority  QueuePriorityConfig  `yaml:"queuepriority" json:"queuepriority"`
	GeoPrice       GeoPriceConfig       `yaml:"geoprice" json:"geoprice"`
	TokenAuction   TokenAuctionConfig   `yaml:"tokenauction" json:"tokenauction"`
	CanaryDeploy   CanaryDeployConfig   `yaml:"canarydeploy" json:"canarydeploy"`
	PlaybackStudio PlaybackStudioConfig `yaml:"playbackstudio" json:"playbackstudio"`
	WebhookForge   WebhookForgeConfig   `yaml:"webhookforge" json:"webhookforge"`
}

// Duration is a time.Duration that can be unmarshaled from YAML strings like "30s", "5m", "1h".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", s, err)
		}
		d.Duration = parsed
		return nil
	}
	// Try as integer (nanoseconds)
	var ns int64
	if err := value.Decode(&ns); err == nil {
		d.Duration = time.Duration(ns)
		return nil
	}
	return fmt.Errorf("cannot parse duration from %q", value.Value)
}

// ProviderConfig holds settings for a single LLM provider.
type ProviderConfig struct {
	APIKey     string   `yaml:"api_key" json:"api_key"`
	BaseURL    string   `yaml:"base_url" json:"base_url"`
	Timeout    Duration `yaml:"timeout" json:"timeout"`
	MaxRetries int      `yaml:"max_retries" json:"max_retries"`
}

// ProjectConfig holds per-project settings.
type ProjectConfig struct {
	Provider string      `yaml:"provider" json:"provider"`
	Model    string      `yaml:"model" json:"model"`
	Caps     CapsConfig  `yaml:"caps" json:"caps"`
	Alerts   AlertsConfig `yaml:"alerts" json:"alerts"`
}

// CapsConfig holds spend cap settings for a project.
type CapsConfig struct {
	Daily   float64 `yaml:"daily" json:"daily"`
	Monthly float64 `yaml:"monthly" json:"monthly"`
}

// AlertsConfig holds alert settings for a project.
type AlertsConfig struct {
	Webhook    string    `yaml:"webhook" json:"webhook"`
	Thresholds []float64 `yaml:"thresholds" json:"thresholds"`
}

// CacheConfig holds caching settings.
type CacheConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	Strategy   string   `yaml:"strategy" json:"strategy"`
	TTL        Duration `yaml:"ttl" json:"ttl"`
	MaxEntries int      `yaml:"max_entries" json:"max_entries"`
}

// ValidationConfig holds schema validation settings.
type ValidationConfig struct {
	Enabled    bool                `yaml:"enabled" json:"enabled"`
	MaxRetries int                 `yaml:"max_retries" json:"max_retries"`
	Schemas    map[string]any      `yaml:"schemas" json:"schemas"`
}

// FailoverConfig holds failover routing settings.
type FailoverConfig struct {
	Enabled        bool           `yaml:"enabled" json:"enabled"`
	Strategy       string         `yaml:"strategy" json:"strategy"`
	Providers      []string       `yaml:"providers" json:"providers"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
}

// CircuitBreakerConfig holds circuit breaker settings.
type CircuitBreakerConfig struct {
	FailureThreshold int      `yaml:"failure_threshold" json:"failure_threshold"`
	RecoveryTimeout  Duration `yaml:"recovery_timeout" json:"recovery_timeout"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`
	Default   RateLimitRule   `yaml:"default" json:"default"`
	PerIP     bool            `yaml:"per_ip" json:"per_ip"`
	PerUser   bool            `yaml:"per_user" json:"per_user"`
	Abuse     AbuseConfig     `yaml:"abuse_detection" json:"abuse_detection"`
}

// RateLimitRule holds a single rate limit rule.
type RateLimitRule struct {
	RequestsPerMinute int `yaml:"requests_per_minute" json:"requests_per_minute"`
	RequestsPerHour   int `yaml:"requests_per_hour" json:"requests_per_hour"`
	Burst             int `yaml:"burst" json:"burst"`
}

// AbuseConfig holds abuse detection settings.
type AbuseConfig struct {
	Enabled            bool `yaml:"enabled" json:"enabled"`
	DuplicateThreshold int  `yaml:"duplicate_threshold" json:"duplicate_threshold"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	StoreBodies   bool `yaml:"store_bodies" json:"store_bodies"`
	MaxBodySize   int  `yaml:"max_body_size" json:"max_body_size"`
	RetentionDays int  `yaml:"retention_days" json:"retention_days"`
}

// KeyPoolConfig holds API key pooling settings.
type KeyPoolConfig struct {
	Enabled  bool             `yaml:"enabled" json:"enabled"`
	Strategy string           `yaml:"strategy" json:"strategy"` // round-robin, least-used, random
	Cooldown Duration         `yaml:"cooldown" json:"cooldown"`
	Keys     []PooledKeyEntry `yaml:"keys" json:"keys"`
}

// PooledKeyEntry represents a single API key in the pool.
type PooledKeyEntry struct {
	Name     string `yaml:"name" json:"name"`
	Key      string `yaml:"key" json:"key"`
	Provider string `yaml:"provider" json:"provider"`
	Weight   int    `yaml:"weight" json:"weight"`
}

// PromptGuardConfig holds PII redaction and injection detection settings.
type PromptGuardConfig struct {
	Enabled   bool             `yaml:"enabled" json:"enabled"`
	PII       PIIConfig        `yaml:"pii" json:"pii"`
	Injection InjectionConfig  `yaml:"injection" json:"injection"`
}

// PIIConfig holds PII detection settings.
type PIIConfig struct {
	Mode     string            `yaml:"mode" json:"mode"` // redact, redact-restore, block
	Patterns []string          `yaml:"patterns" json:"patterns"`
	Custom   []CustomPIIPattern `yaml:"custom" json:"custom"`
}

// CustomPIIPattern defines a custom PII regex pattern.
type CustomPIIPattern struct {
	Name    string `yaml:"name" json:"name"`
	Pattern string `yaml:"pattern" json:"pattern"`
}

// InjectionConfig holds prompt injection detection settings.
type InjectionConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Sensitivity string `yaml:"sensitivity" json:"sensitivity"` // low, medium, high
	Action      string `yaml:"action" json:"action"`           // block, warn, log
}

// ModelSwitchConfig holds smart model routing settings.
type ModelSwitchConfig struct {
	Enabled bool               `yaml:"enabled" json:"enabled"`
	Rules   []ModelRouteRule   `yaml:"rules" json:"rules"`
	Default string             `yaml:"default" json:"default"`
}

// ModelRouteRule defines a single model routing rule.
type ModelRouteRule struct {
	Name       string `yaml:"name" json:"name"`
	Condition  string `yaml:"condition" json:"condition"` // token_count, pattern, header, cost
	Operator   string `yaml:"operator" json:"operator"`   // gt, lt, eq, contains, matches
	Value      string `yaml:"value" json:"value"`
	Model      string `yaml:"model" json:"model"`
	Provider   string `yaml:"provider" json:"provider"`
	Weight     int    `yaml:"weight" json:"weight"` // for A/B testing (0-100)
}

// EvalGateConfig holds response quality scoring settings.
type EvalGateConfig struct {
	Enabled    bool              `yaml:"enabled" json:"enabled"`
	Validators []ValidatorConfig `yaml:"validators" json:"validators"`
	RetryBudget int             `yaml:"retry_budget" json:"retry_budget"`
}

// ValidatorConfig defines a single response validator.
type ValidatorConfig struct {
	Name   string `yaml:"name" json:"name"`     // json_parse, min_length, max_length, regex, contains
	Params string `yaml:"params" json:"params"` // validator-specific params (e.g., regex pattern, min chars)
	Action string `yaml:"action" json:"action"` // retry, warn, log
}

// UsagePulseConfig holds per-user/feature metering settings.
type UsagePulseConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	Dimensions []string `yaml:"dimensions" json:"dimensions"` // user, feature, team, project, custom
	Caps       []UsageCapRule `yaml:"caps" json:"caps"`
	ExportFormat string `yaml:"export_format" json:"export_format"` // csv, json
}

// UsageCapRule defines a spend cap for a dimension.
type UsageCapRule struct {
	Dimension string  `yaml:"dimension" json:"dimension"`
	Key       string  `yaml:"key" json:"key"`
	Daily     float64 `yaml:"daily" json:"daily"`
	Monthly   float64 `yaml:"monthly" json:"monthly"`
}

// ─── Phase 2 Config Types ────────────────────────────────────────────────────

// PromptPadConfig holds prompt template management settings.
type PromptPadConfig struct {
	Enabled   bool                `yaml:"enabled" json:"enabled"`
	Templates []PromptTemplate    `yaml:"templates" json:"templates"`
	Storage   string              `yaml:"storage" json:"storage"`
	APIPrefix string              `yaml:"api_prefix" json:"api_prefix"`
}

// PromptTemplate defines a versioned prompt template.
type PromptTemplate struct {
	Name     string           `yaml:"name" json:"name"`
	Version  string           `yaml:"version" json:"version"`
	Template string           `yaml:"template" json:"template"`
	Variants []PromptVariant  `yaml:"variants" json:"variants"`
}

// PromptVariant defines an A/B test variant.
type PromptVariant struct {
	Name     string `yaml:"name" json:"name"`
	Weight   int    `yaml:"weight" json:"weight"`
	Override string `yaml:"override" json:"override"`
}

// TokenTrimConfig holds context window optimization settings.
type TokenTrimConfig struct {
	Enabled       bool                   `yaml:"enabled" json:"enabled"`
	DefaultStrat  string                 `yaml:"default_strategy" json:"default_strategy"`
	SafetyMargin  int                    `yaml:"safety_margin" json:"safety_margin"`
	Models        map[string]TrimModel   `yaml:"models" json:"models"`
	Protect       []string               `yaml:"protect" json:"protect"`
}

// TrimModel defines per-model trim settings.
type TrimModel struct {
	MaxContext int    `yaml:"max_context" json:"max_context"`
	Strategy   string `yaml:"strategy" json:"strategy"`
}

// BatchQueueConfig holds async batching settings.
type BatchQueueConfig struct {
	Enabled     bool              `yaml:"enabled" json:"enabled"`
	Concurrency BatchConcurrency  `yaml:"concurrency" json:"concurrency"`
	Retry       BatchRetryConfig  `yaml:"retry" json:"retry"`
	Delivery    BatchDelivery     `yaml:"delivery" json:"delivery"`
	Priorities  []string          `yaml:"priority_levels" json:"priority_levels"`
	Retention   Duration          `yaml:"retention" json:"retention"`
}

// BatchConcurrency holds concurrency limits.
type BatchConcurrency struct {
	Default     int            `yaml:"default" json:"default"`
	PerProvider map[string]int `yaml:"per_provider" json:"per_provider"`
}

// BatchRetryConfig holds batch retry settings.
type BatchRetryConfig struct {
	MaxAttempts int    `yaml:"max_attempts" json:"max_attempts"`
	Backoff     string `yaml:"backoff" json:"backoff"`
	OnRateLimit string `yaml:"on_rate_limit" json:"on_rate_limit"`
}

// BatchDelivery holds result delivery settings.
type BatchDelivery struct {
	Mode        string `yaml:"mode" json:"mode"`
	WebhookURL  string `yaml:"webhook_url" json:"webhook_url"`
	WebhookRetry int   `yaml:"webhook_retry" json:"webhook_retry"`
}

// MultiCallConfig holds multi-model consensus settings.
type MultiCallConfig struct {
	Enabled  bool             `yaml:"enabled" json:"enabled"`
	Routes   []MultiCallRoute `yaml:"routes" json:"routes"`
	Fallback string           `yaml:"fallback_on_timeout" json:"fallback_on_timeout"`
}

// MultiCallRoute defines a multi-model route.
type MultiCallRoute struct {
	Name       string   `yaml:"name" json:"name"`
	Models     []string `yaml:"models" json:"models"`
	Strategy   string   `yaml:"strategy" json:"strategy"`
	Timeout    Duration `yaml:"timeout" json:"timeout"`
	OnDisagree string   `yaml:"on_disagree" json:"on_disagree"`
	MinQuality float64  `yaml:"min_quality" json:"min_quality"`
}

// StreamSnapConfig holds SSE capture settings.
type StreamSnapConfig struct {
	Enabled   bool             `yaml:"enabled" json:"enabled"`
	Retention Duration         `yaml:"retention" json:"retention"`
	MaxSize   string           `yaml:"max_size" json:"max_size"`
	Metrics   StreamMetrics    `yaml:"metrics" json:"metrics"`
	Replay    StreamReplay     `yaml:"replay" json:"replay"`
}

// StreamMetrics defines which stream metrics to capture.
type StreamMetrics struct {
	TTFT            bool `yaml:"ttft" json:"ttft"`
	TPS             bool `yaml:"tps" json:"tps"`
	CompletionCheck bool `yaml:"completion_check" json:"completion_check"`
}

// StreamReplay defines replay settings.
type StreamReplay struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Auth    string `yaml:"auth" json:"auth"`
}

// LLMTapConfig holds analytics portal settings.
type LLMTapConfig struct {
	Enabled    bool          `yaml:"enabled" json:"enabled"`
	Percentiles []int        `yaml:"latency_percentiles" json:"latency_percentiles"`
	Granularity string       `yaml:"cost_granularity" json:"cost_granularity"`
	Retention   Duration     `yaml:"retention" json:"retention"`
	Alerts      []TapAlert   `yaml:"alerts" json:"alerts"`
	Embed       TapEmbed     `yaml:"embed" json:"embed"`
}

// TapAlert defines an analytics alert.
type TapAlert struct {
	Name      string `yaml:"name" json:"name"`
	Condition string `yaml:"condition" json:"condition"`
	Window    string `yaml:"window" json:"window"`
	Notify    string `yaml:"notify" json:"notify"`
}

// TapEmbed defines embeddable chart settings.
type TapEmbed struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
}

// ContextPackConfig holds RAG-without-vector-DB settings.
type ContextPackConfig struct {
	Enabled   bool                  `yaml:"enabled" json:"enabled"`
	Sources   []ContextSource       `yaml:"sources" json:"sources"`
	Injection ContextInjection      `yaml:"injection" json:"injection"`
}

// ContextSource defines a context data source.
type ContextSource struct {
	Name      string   `yaml:"name" json:"name"`
	Type      string   `yaml:"type" json:"type"`
	Path      string   `yaml:"path" json:"path"`
	URL       string   `yaml:"url" json:"url"`
	Content   string   `yaml:"content" json:"content"` // for inline type
	Query     string   `yaml:"query" json:"query"`
	Match     string   `yaml:"match" json:"match"`
	Patterns  []string `yaml:"patterns" json:"patterns"`
	ChunkSize int      `yaml:"chunk_size" json:"chunk_size"`
	Overlap   int      `yaml:"overlap" json:"overlap"`
	Refresh   Duration `yaml:"refresh" json:"refresh"`
}

// ContextInjection defines how context is injected into requests.
type ContextInjection struct {
	Position  string `yaml:"position" json:"position"`
	MaxTokens int    `yaml:"max_tokens" json:"max_tokens"`
	Template  string `yaml:"template" json:"template"`
}

// RetryPilotConfig holds intelligent retry settings.
type RetryPilotConfig struct {
	Enabled        bool                  `yaml:"enabled" json:"enabled"`
	MaxRetries     int                   `yaml:"max_retries" json:"max_retries"`
	Backoff        string                `yaml:"backoff" json:"backoff"`
	BaseDelay      Duration              `yaml:"base_delay" json:"base_delay"`
	MaxDelay       Duration              `yaml:"max_delay" json:"max_delay"`
	Jitter         string                `yaml:"jitter" json:"jitter"`
	CircuitBreaker RetryCircuitBreaker   `yaml:"circuit_breaker" json:"circuit_breaker"`
	Deadline       RetryDeadline         `yaml:"deadline" json:"deadline"`
	Downgrade      RetryDowngrade        `yaml:"downgrade" json:"downgrade"`
	Budget         RetryBudget           `yaml:"budget" json:"budget"`
}

// RetryCircuitBreaker holds circuit breaker settings for RetryPilot.
type RetryCircuitBreaker struct {
	FailureThreshold int      `yaml:"failure_threshold" json:"failure_threshold"`
	RecoveryTimeout  Duration `yaml:"recovery_timeout" json:"recovery_timeout"`
	HalfOpenRequests int      `yaml:"half_open_requests" json:"half_open_requests"`
}

// RetryDeadline holds deadline-aware retry settings.
type RetryDeadline struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	MinRemaining Duration `yaml:"min_remaining" json:"min_remaining"`
}

// RetryDowngrade holds model downgrade settings.
type RetryDowngrade struct {
	Enabled       bool              `yaml:"enabled" json:"enabled"`
	AfterFailures int               `yaml:"after_failures" json:"after_failures"`
	DowngradeMap  map[string]string `yaml:"downgrade_map" json:"downgrade_map"`
}

// RetryBudget holds retry budget settings.
type RetryBudget struct {
	MaxPerMinute int `yaml:"max_retries_per_minute" json:"max_retries_per_minute"`
}

// ─── Phase 3 Config Types ────────────────────────────────────────────────────

// ToxicFilterConfig holds content moderation settings.
type ToxicFilterConfig struct {
	Enabled    bool                `yaml:"enabled" json:"enabled"`
	ScanInput  bool                `yaml:"scan_input" json:"scan_input"`
	ScanOutput bool                `yaml:"scan_output" json:"scan_output"`
	Action     string              `yaml:"action" json:"action"` // block, redact, flag
	Categories []ToxicCategory     `yaml:"categories" json:"categories"`
	Custom     []ToxicCustomRule   `yaml:"custom" json:"custom"`
	Webhook    string              `yaml:"webhook" json:"webhook"`
}

// ToxicCategory defines a built-in moderation category.
type ToxicCategory struct {
	Name    string `yaml:"name" json:"name"` // harmful, hate_speech, violence, self_harm, sexual, profanity
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Action  string `yaml:"action" json:"action"` // override per-category action
}

// ToxicCustomRule defines a custom moderation rule.
type ToxicCustomRule struct {
	Name    string `yaml:"name" json:"name"`
	Pattern string `yaml:"pattern" json:"pattern"` // regex
	Action  string `yaml:"action" json:"action"`
}

// ComplianceLogConfig holds immutable audit trail settings.
type ComplianceLogConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	HashAlgorithm   string   `yaml:"hash_algorithm" json:"hash_algorithm"` // sha256
	RetentionDays   int      `yaml:"retention_days" json:"retention_days"`
	ExportFormats   []string `yaml:"export_formats" json:"export_formats"` // json, csv, soc2
	IncludeHeaders  bool     `yaml:"include_headers" json:"include_headers"`
	IncludeBodies   bool     `yaml:"include_bodies" json:"include_bodies"`
	MaxBodySize     int      `yaml:"max_body_size" json:"max_body_size"`
	VerifyOnStartup bool     `yaml:"verify_on_startup" json:"verify_on_startup"`
}

// SecretScanConfig holds secret detection settings.
type SecretScanConfig struct {
	Enabled      bool                `yaml:"enabled" json:"enabled"`
	ScanInput    bool                `yaml:"scan_input" json:"scan_input"`
	ScanOutput   bool                `yaml:"scan_output" json:"scan_output"`
	Action       string              `yaml:"action" json:"action"` // block, redact, alert
	Patterns     []string            `yaml:"patterns" json:"patterns"` // builtin pattern names
	Custom       []SecretCustomRule  `yaml:"custom" json:"custom"`
	MaskPreview  bool                `yaml:"mask_preview" json:"mask_preview"` // show first4+last4
	Webhook      string              `yaml:"webhook" json:"webhook"`
}

// SecretCustomRule defines a custom secret detection pattern.
type SecretCustomRule struct {
	Name     string `yaml:"name" json:"name"`
	Pattern  string `yaml:"pattern" json:"pattern"`
	Severity string `yaml:"severity" json:"severity"` // critical, high, medium, low
}

// TraceLinkConfig holds distributed tracing settings.
type TraceLinkConfig struct {
	Enabled      bool    `yaml:"enabled" json:"enabled"`
	SampleRate   float64 `yaml:"sample_rate" json:"sample_rate"` // 0.0-1.0
	PropagateW3C bool    `yaml:"propagate_w3c" json:"propagate_w3c"`
	ServiceName  string  `yaml:"service_name" json:"service_name"`
	MaxSpans     int     `yaml:"max_spans" json:"max_spans"`
	ExportOTLP   string  `yaml:"export_otlp" json:"export_otlp"` // OTLP endpoint URL
}

// IPFenceConfig holds IP allowlisting and geofencing settings.
type IPFenceConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Mode      string   `yaml:"mode" json:"mode"`           // allowlist, denylist, mixed
	Action    string   `yaml:"action" json:"action"`       // block, log, warn
	Allowlist []string `yaml:"allowlist" json:"allowlist"`  // IPs, CIDR ranges
	Denylist  []string `yaml:"denylist" json:"denylist"`    // IPs, CIDR ranges
	TrustProxy bool   `yaml:"trust_proxy" json:"trust_proxy"` // trust X-Forwarded-For
	LogBlocked bool   `yaml:"log_blocked" json:"log_blocked"`
	Webhook    string  `yaml:"webhook" json:"webhook"`
}

// EmbedCacheConfig holds embedding response caching settings.
type EmbedCacheConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	MaxEntries int      `yaml:"max_entries" json:"max_entries"`
	TTL        Duration `yaml:"ttl" json:"ttl"`
	Models     []string `yaml:"models" json:"models"` // models to cache (empty = all)
}

// AnthroFitConfig holds Anthropic deep-compatibility settings.
type AnthroFitConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	SystemPromptMode  string `yaml:"system_prompt_mode" json:"system_prompt_mode"`   // separate, merge, auto
	ToolTranslation   bool   `yaml:"tool_translation" json:"tool_translation"`       // translate tool schemas
	StreamNormalize   bool   `yaml:"stream_normalize" json:"stream_normalize"`       // normalize SSE format
	MaxTokensDefault  int    `yaml:"max_tokens_default" json:"max_tokens_default"`   // default max_tokens for Anthropic
	CacheControl      bool   `yaml:"cache_control" json:"cache_control"`             // enable prompt caching headers
}

// AlertPulseConfig holds alerting engine settings.
type AlertPulseConfig struct {
	Enabled        bool        `yaml:"enabled" json:"enabled"`
	Rules          []AlertRule `yaml:"rules" json:"rules"`
	DefaultWebhook string      `yaml:"default_webhook" json:"default_webhook"`
	WindowDuration Duration    `yaml:"window_duration" json:"window_duration"`
	Cooldown       Duration    `yaml:"cooldown" json:"cooldown"`
}

// AlertRule defines a single alerting rule with metric, threshold, and channel.
type AlertRule struct {
	Name       string  `yaml:"name" json:"name"`
	Metric     string  `yaml:"metric" json:"metric"`         // error_rate, latency_p95, latency_p50, cost_per_min
	Threshold  float64 `yaml:"threshold" json:"threshold"`
	Channel    string  `yaml:"channel" json:"channel"`       // webhook, log
	WebhookURL string  `yaml:"webhook_url" json:"webhook_url"`
}

// ChatMemConfig holds conversation memory management settings.
type ChatMemConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	Strategy     string   `yaml:"strategy" json:"strategy"`           // sliding_window, importance, default (simple truncation)
	MaxMessages  int      `yaml:"max_messages" json:"max_messages"`   // max messages per session
	InjectMemory bool     `yaml:"inject_memory" json:"inject_memory"` // inject history into requests
	SessionTTL   Duration `yaml:"session_ttl" json:"session_ttl"`     // auto-expire idle sessions
}

// MockLLMConfig holds mock LLM server settings.
type MockLLMConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	Fixtures        []MockFixture `yaml:"fixtures" json:"fixtures"`
	DefaultResponse string        `yaml:"default_response" json:"default_response"` // response when no fixture matches
	Passthrough     bool          `yaml:"passthrough" json:"passthrough"`           // pass to real provider if no match
}

// MockFixture defines a canned response matched by prompt content.
type MockFixture struct {
	Name         string `yaml:"name" json:"name"`
	MatchType    string `yaml:"match_type" json:"match_type"`       // exact, contains, regex, any
	Pattern      string `yaml:"pattern" json:"pattern"`             // match pattern
	Model        string `yaml:"model" json:"model"`                 // optional model filter
	Response     string `yaml:"response" json:"response"`           // canned response content
	DelayMs      int    `yaml:"delay_ms" json:"delay_ms"`           // simulated latency
	ErrorCode    int    `yaml:"error_code" json:"error_code"`       // simulate error (0 = success)
	ErrorMessage string `yaml:"error_message" json:"error_message"` // error message when simulating errors
}

// TenantWallConfig holds multi-tenant isolation settings.
type TenantWallConfig struct {
	Enabled              bool           `yaml:"enabled" json:"enabled"`
	RequireTenant        bool           `yaml:"require_tenant" json:"require_tenant"`               // reject requests without tenant ID
	UseProjectAsTenant   bool           `yaml:"use_project_as_tenant" json:"use_project_as_tenant"` // use Project field as tenant ID
	KeyPrefixMode        bool           `yaml:"key_prefix_mode" json:"key_prefix_mode"`             // extract tenant from key prefix
	WindowDuration       Duration       `yaml:"window_duration" json:"window_duration"`
	DefaultMaxRequests   int            `yaml:"default_max_requests" json:"default_max_requests"`     // default per-tenant request limit
	DefaultMaxSpend      float64        `yaml:"default_max_spend" json:"default_max_spend"`           // default per-tenant spend cap
	DefaultAllowedModels []string       `yaml:"default_allowed_models" json:"default_allowed_models"` // default model allowlist
	Tenants              []TenantConfig `yaml:"tenants" json:"tenants"`
}

// TenantConfig defines per-tenant overrides.
type TenantConfig struct {
	ID                   string   `yaml:"id" json:"id"`
	MaxRequestsPerWindow int      `yaml:"max_requests_per_window" json:"max_requests_per_window"`
	MaxSpendPerWindow    float64  `yaml:"max_spend_per_window" json:"max_spend_per_window"`
	AllowedModels        []string `yaml:"allowed_models" json:"allowed_models"`
}

// IdleKillConfig holds runaway request termination settings.
type IdleKillConfig struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	MaxDuration        Duration `yaml:"max_duration" json:"max_duration"`                 // max request duration before kill
	MaxTokensPerRequest int     `yaml:"max_tokens_per_request" json:"max_tokens_per_request"` // max tokens before kill
	MaxCostPerRequest  float64  `yaml:"max_cost_per_request" json:"max_cost_per_request"` // max estimated cost before kill
	LoopDetection      bool     `yaml:"loop_detection" json:"loop_detection"`             // detect repeated identical requests
	LoopWindow         Duration `yaml:"loop_window" json:"loop_window"`                   // time window for loop detection
	LoopThreshold      int      `yaml:"loop_threshold" json:"loop_threshold"`             // repeated requests to trigger kill
	WebhookURL         string   `yaml:"webhook_url" json:"webhook_url"`                   // alert on kill events
}

// AgentGuardConfig holds agent session safety rail settings.
type AgentGuardConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	MaxCalls      int      `yaml:"max_calls" json:"max_calls"`
	MaxCost       float64  `yaml:"max_cost" json:"max_cost"`
	MaxDuration   Duration `yaml:"max_duration" json:"max_duration"`
	SessionHeader string   `yaml:"session_header" json:"session_header"`
	WebhookURL    string   `yaml:"webhook_url" json:"webhook_url"`
}

// CodeFenceConfig holds code validation settings.
type CodeFenceConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	ForbiddenPatterns []string `yaml:"forbidden_patterns" json:"forbidden_patterns"`
	MaxComplexity     int      `yaml:"max_complexity" json:"max_complexity"`
	Action            string   `yaml:"action" json:"action"`
}

// HalluciCheckConfig holds hallucination checking settings.
type HalluciCheckConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	CheckURLs   bool   `yaml:"check_urls" json:"check_urls"`
	CheckEmails bool   `yaml:"check_emails" json:"check_emails"`
	Action      string `yaml:"action" json:"action"`
}

// TierDropConfig holds model downgrade settings.
type TierDropConfig struct {
	Enabled bool           `yaml:"enabled" json:"enabled"`
	Tiers   []TierDropTier `yaml:"tiers" json:"tiers"`
}

// TierDropTier defines a cost threshold for model downgrade.
type TierDropTier struct {
	Threshold float64 `yaml:"threshold" json:"threshold"`
	Model     string  `yaml:"model" json:"model"`
}

// DriftWatchConfig holds model drift detection settings.
type DriftWatchConfig struct {
	Enabled        bool    `yaml:"enabled" json:"enabled"`
	DriftThreshold float64 `yaml:"drift_threshold" json:"drift_threshold"`
	WebhookURL     string  `yaml:"webhook_url" json:"webhook_url"`
}

// FeedbackLoopConfig holds user feedback collection settings.
type FeedbackLoopConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

// ABRouterConfig holds A/B testing experiment settings.
type ABRouterConfig struct {
	Enabled     bool           `yaml:"enabled" json:"enabled"`
	Experiments []ABExperiment `yaml:"experiments" json:"experiments"`
}

// ABExperiment defines an A/B test experiment.
type ABExperiment struct {
	Name     string      `yaml:"name" json:"name"`
	Variants []ABVariant `yaml:"variants" json:"variants"`
}

// ABVariant defines a variant in an A/B experiment.
type ABVariant struct {
	Name   string  `yaml:"name" json:"name"`
	Weight float64 `yaml:"weight" json:"weight"`
	Model  string  `yaml:"model" json:"model"`
}

// GuardRailConfig holds topic fencing settings.
type GuardRailConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	AllowedTopics []string `yaml:"allowed_topics" json:"allowed_topics"`
	DeniedTopics  []string `yaml:"denied_topics" json:"denied_topics"`
	FallbackMsg   string   `yaml:"fallback_message" json:"fallback_message"`
}

// GeminiShimConfig holds Gemini compatibility settings.
type GeminiShimConfig struct {
	Enabled         bool `yaml:"enabled" json:"enabled"`
	AutoRetrySafety bool `yaml:"auto_retry_safety" json:"auto_retry_safety"`
	NormalizeTokens bool `yaml:"normalize_tokens" json:"normalize_tokens"`
}

// LocalSyncConfig holds local/cloud model blending settings.
type LocalSyncConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	LocalEndpoint   string `yaml:"local_endpoint" json:"local_endpoint"`
	FallbackToCloud bool   `yaml:"fallback_to_cloud" json:"fallback_to_cloud"`
}

// DevProxyConfig holds developer debugging proxy settings.
type DevProxyConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	LogHeaders bool `yaml:"log_headers" json:"log_headers"`
	LogBodies  bool `yaml:"log_bodies" json:"log_bodies"`
}

// PromptSlimConfig holds prompt compression settings.
type PromptSlimConfig struct {
	Enabled        bool    `yaml:"enabled" json:"enabled"`
	Aggressiveness float64 `yaml:"aggressiveness" json:"aggressiveness"`
}

// PromptLintConfig holds prompt quality analysis settings.
type PromptLintConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	BlockOnFail bool `yaml:"block_on_fail" json:"block_on_fail"`
}

// ApprovalGateConfig holds prompt change approval settings.
type ApprovalGateConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Approvers []string `yaml:"approvers" json:"approvers"`
}

// OutputCapConfig holds output length capping settings.
type OutputCapConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`
	MaxChars int  `yaml:"max_chars" json:"max_chars"`
}

// AgeGateConfig holds child safety filtering settings.
type AgeGateConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Tier    string `yaml:"tier" json:"tier"`
}

// VoiceBridgeConfig holds voice pipeline optimization settings.
type VoiceBridgeConfig struct {
	Enabled   bool `yaml:"enabled" json:"enabled"`
	MaxLength int  `yaml:"max_length" json:"max_length"`
}

// ImageProxyConfig holds image generation proxy settings.
type ImageProxyConfig struct {
	Enabled      bool `yaml:"enabled" json:"enabled"`
	CacheEnabled bool `yaml:"cache_enabled" json:"cache_enabled"`
}

// LangBridgeConfig holds cross-language translation settings.
type LangBridgeConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	TargetLang string `yaml:"target_lang" json:"target_lang"`
}

// ContextWindowConfig holds context window debugging settings.
type ContextWindowConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// RegionRouteConfig holds data residency routing settings.
type RegionRouteConfig struct {
	Enabled bool             `yaml:"enabled" json:"enabled"`
	Routes  []RegionRouteDef `yaml:"routes" json:"routes"`
}

// RegionRouteDef defines a region-to-endpoint mapping.
type RegionRouteDef struct {
	Region   string `yaml:"region" json:"region"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Provider string `yaml:"provider" json:"provider"`
}

// ChainForgeConfig holds multi-step LLM pipeline settings.
type ChainForgeConfig struct {
	Enabled    bool              `yaml:"enabled" json:"enabled"`
	Pipelines  []ChainForgePipe  `yaml:"pipelines" json:"pipelines"`
}

// ChainForgePipe defines a named pipeline with steps.
type ChainForgePipe struct {
	Name  string   `yaml:"name" json:"name"`
	Steps []string `yaml:"steps" json:"steps"`
}

// CronLLMConfig holds scheduled LLM task settings.
type CronLLMConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Jobs    []CronLLMJob  `yaml:"jobs" json:"jobs"`
}

// CronLLMJob defines a scheduled LLM task.
type CronLLMJob struct {
	Name     string `yaml:"name" json:"name"`
	Schedule string `yaml:"schedule" json:"schedule"`
	Prompt   string `yaml:"prompt" json:"prompt"`
	Model    string `yaml:"model" json:"model"`
}

// WebhookRelayConfig holds webhook-to-LLM relay settings.
type WebhookRelayConfig struct {
	Enabled  bool                `yaml:"enabled" json:"enabled"`
	Triggers []WebhookTrigger    `yaml:"triggers" json:"triggers"`
}

// WebhookTrigger defines a webhook trigger mapping.
type WebhookTrigger struct {
	Path     string `yaml:"path" json:"path"`
	Prompt   string `yaml:"prompt" json:"prompt"`
	Model    string `yaml:"model" json:"model"`
}

// BillSyncConfig holds per-customer billing settings.
type BillSyncConfig struct {
	Enabled    bool    `yaml:"enabled" json:"enabled"`
	MarkupPct  float64 `yaml:"markup_pct" json:"markup_pct"`
	Currency   string  `yaml:"currency" json:"currency"`
}

// WhiteLabelConfig holds custom branding settings.
type WhiteLabelConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	BrandName string `yaml:"brand_name" json:"brand_name"`
	LogoURL   string `yaml:"logo_url" json:"logo_url"`
	CustomCSS string `yaml:"custom_css" json:"custom_css"`
}

// TrainExportConfig holds training data export settings.
type TrainExportConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Format   string `yaml:"format" json:"format"`
	MaxPairs int    `yaml:"max_pairs" json:"max_pairs"`
}

// SynthGenConfig holds synthetic data generation settings.
type SynthGenConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	BatchSize  int    `yaml:"batch_size" json:"batch_size"`
}

// DiffPromptConfig holds prompt diff settings.
type DiffPromptConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// LLMBenchConfig holds model benchmarking settings.
type LLMBenchConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// MaskModeConfig holds demo masking settings.
type MaskModeConfig struct {
	Enabled   bool `yaml:"enabled" json:"enabled"`
	MaskNames bool `yaml:"mask_names" json:"mask_names"`
	MaskEmail bool `yaml:"mask_email" json:"mask_email"`
	MaskPhone bool `yaml:"mask_phone" json:"mask_phone"`
}

// TokenMarketConfig holds budget pool settings.
type TokenMarketConfig struct {
	Enabled bool              `yaml:"enabled" json:"enabled"`
	Pools   []TokenMarketPool `yaml:"pools" json:"pools"`
}

// TokenMarketPool defines a named budget pool.
type TokenMarketPool struct {
	Name   string  `yaml:"name" json:"name"`
	Budget float64 `yaml:"budget" json:"budget"`
}

// LLMSyncConfig holds config sync settings.
type LLMSyncConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Environment string `yaml:"environment" json:"environment"`
}

// ClusterModeConfig holds multi-instance settings.
type ClusterModeConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	NodeID   string `yaml:"node_id" json:"node_id"`
	Peers    []string `yaml:"peers" json:"peers"`
}

// EncryptVaultConfig holds encryption settings.
type EncryptVaultConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Key     string `yaml:"key" json:"key"`
}

// MirrorTestConfig holds shadow testing settings.
type MirrorTestConfig struct {
	Enabled     bool    `yaml:"enabled" json:"enabled"`
	ShadowModel string  `yaml:"shadow_model" json:"shadow_model"`
	SampleRate  float64 `yaml:"sample_rate" json:"sample_rate"`
}

// envVarRegex matches ${VAR_NAME} patterns.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses a YAML config file with env var interpolation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Interpolate environment variables
	content := envVarRegex.ReplaceAllStringFunc(string(data), func(match string) string {
		varName := match[2 : len(match)-1] // Strip ${ and }
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match // Leave as-is if not set
	})

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply defaults
	if cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DataDir = home + "/.stockyard"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return &cfg, nil
}

// LoadOrDefault loads a config file if it exists, otherwise returns defaults for the product.
func LoadOrDefault(path, product string) (*Config, error) {
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}

	// Check common locations
	candidates := []string{
		product + ".yaml",
		product + ".yml",
		"config.yaml",
		"config.yml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return Load(c)
		}
	}

	// Return defaults
	return DefaultConfig(product), nil
}

// Validate checks the config for required fields and returns errors.
func (c *Config) Validate() error {
	if c.Port == 0 {
		return fmt.Errorf("port is required")
	}

	// Check that at least one provider has an API key
	hasProvider := false
	for name, p := range c.Providers {
		if p.APIKey != "" && !strings.HasPrefix(p.APIKey, "${") {
			hasProvider = true
			break
		}
		_ = name
	}
	if !hasProvider && len(c.Providers) > 0 {
		return fmt.Errorf("at least one provider needs an API key (set via environment variable)")
	}

	return nil
}
