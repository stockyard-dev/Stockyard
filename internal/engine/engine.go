// Package engine provides shared bootstrap logic for all Stockyard products.
// Each cmd/*/main.go calls engine.Boot() with product-specific options.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/api"
	"github.com/stockyard-dev/stockyard/internal/apiserver"
	"github.com/stockyard-dev/stockyard/internal/apps/observe"
	"github.com/stockyard-dev/stockyard/internal/auth"
	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/dashboard"
	"github.com/stockyard-dev/stockyard/internal/features"
	"github.com/stockyard-dev/stockyard/internal/license"
	"github.com/stockyard-dev/stockyard/internal/platform"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/site"
	"github.com/stockyard-dev/stockyard/internal/slog"
	"github.com/stockyard-dev/stockyard/internal/storage"
	"github.com/stockyard-dev/stockyard/internal/toggle"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// Features flags control which middleware is enabled per product.
type Features struct {
	SpendTracking  bool
	SpendCaps      bool
	Alerts         bool
	Cache          bool
	Validation     bool
	Failover       bool
	RateLimiting   bool
	RequestLogging bool // always true, but controls body storage
	FullBodyLog    bool // store full request/response bodies

	// Phase 1 expansion
	KeyPool     bool
	PromptGuard bool
	ModelSwitch bool
	EvalGate    bool
	UsagePulse  bool

	// Phase 2 expansion
	PromptPad   bool
	TokenTrim   bool
	BatchQueue  bool
	MultiCall   bool
	StreamSnap  bool
	LLMTap      bool
	ContextPack bool
	RetryPilot  bool

	// Phase 3 expansion
	ToxicFilter   bool
	ComplianceLog bool
	SecretScan    bool
	TraceLink     bool
	IPFence       bool
	EmbedCache    bool
	AnthroFit     bool
	AlertPulse    bool
	ChatMem       bool
	MockLLM       bool
	TenantWall    bool
	IdleKill      bool

	// Phase 3 P2 expansion
	AgentGuard    bool
	CodeFence     bool
	HalluciCheck  bool
	TierDrop      bool
	DriftWatch    bool
	FeedbackLoop  bool
	ABRouter      bool
	GuardRail     bool
	GeminiShim    bool
	LocalSync     bool
	DevProxy      bool
	PromptSlim    bool
	PromptLint    bool
	ApprovalGate  bool
	OutputCap     bool
	AgeGate       bool
	VoiceBridge   bool
	ImageProxy    bool
	LangBridge    bool
	ContextWindow bool
	RegionRoute   bool

	// Phase 3 P3 expansion
	ChainForge    bool
	CronLLM       bool
	WebhookRelay  bool
	BillSync      bool
	WhiteLabel    bool
	TrainExport   bool
	SynthGen      bool
	DiffPrompt    bool
	LLMBench      bool
	MaskMode      bool
	TokenMarket   bool
	LLMSync       bool
	ClusterMode   bool
	EncryptVault  bool
	MirrorTest    bool

	// Phase 4 expansion
	ExtractML      bool
	TableForge     bool
	ToolRouter     bool
	ToolShield     bool
	ToolMock       bool
	AuthGate       bool
	ScopeGuard     bool
	VisionProxy    bool
	AudioProxy     bool
	DocParse       bool
	FrameGrab      bool
	SessionStore   bool
	ConvoFork      bool
	SlotFill       bool
	SemanticCache  bool
	PartialCache   bool
	StreamCache    bool
	PromptChain    bool
	PromptFuzz     bool
	PromptMarket   bool
	CostPredict    bool
	CostMap        bool
	SpotPrice      bool
	LoadForge      bool
	SnapshotTest   bool
	ChaosLLM       bool
	DataMap        bool
	ConsentGate    bool
	RetentionWipe  bool
	PolicyEngine   bool
	StreamSplit    bool
	StreamThrottle bool
	StreamTransform bool
	ModelAlias     bool
	ParamNorm      bool
	QuotaSync      bool
	ErrorNorm      bool
	CohortTrack    bool
	PromptRank     bool
	AnomalyRadar   bool
	EnvSync        bool
	ProxyLog       bool
	CliDash        bool
	EmbedRouter    bool
	FineTuneTrack  bool
	AgentReplay    bool
	SummarizeGate  bool
	CodeLang       bool
	PersonaSwitch  bool
	WarmPool       bool
	EdgeCache      bool
	QueuePriority  bool
	GeoPrice       bool
	TokenAuction   bool
	CanaryDeploy   bool
	PlaybackStudio bool
	WebhookForge   bool
}

// ProductConfig defines a product's identity and feature set.
type ProductConfig struct {
	Name            string
	Product         string // config key: costcap, llmcache, etc.
	Version         string // set via ldflags at build time
	Features        Features
	Apps            []platform.App // 6 flagship apps (registered in cmd/stockyard/main.go)
	EnableAPIServer bool           // mount sy-api billing/licensing/cloud/exchange routes
}

// Boot initializes and starts a product. This is the single entry point
// that all 7 cmd/*/main.go files call.
func Boot(pc ProductConfig) {
	// Handle --version / -v flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		v := pc.Version
		if v == "" {
			v = "dev"
		}
		fmt.Printf("%s %s\n", pc.Product, v)
		os.Exit(0)
	}

	// Handle --health flag (for Homebrew test, scripts, etc.)
	if len(os.Args) > 1 && (os.Args[1] == "--health" || os.Args[1] == "health") {
		fmt.Println("ok")
		os.Exit(0)
	}

	// Handle doctor subcommand
	if len(os.Args) > 1 && (os.Args[1] == "doctor" || os.Args[1] == "--doctor") {
		RunDoctor(pc)
	}

	log.SetFlags(log.Ltime | log.Lshortfile)

	// Initialize structured logging
	slog.Init(slog.Config{
		Level:  os.Getenv("LOG_LEVEL"),  // debug, info, warn, error (default: info)
		Format: os.Getenv("LOG_FORMAT"), // json or text (default: text)
	})
	if slog.IsJSON() {
		log.SetFlags(0) // JSON handles its own formatting
		slog.Info("structured logging initialized", "level", slog.GetLevel().String(), "format", "json")
	}

	// Load config
	configPath := ""
	if len(os.Args) > 1 && os.Args[1] != "serve" {
		configPath = os.Args[1]
	}
	cfg, err := config.LoadOrDefault(configPath, pc.Product)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Allow PORT env var to override config (Railway, Heroku, etc.)
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}

	// Allow DATA_DIR env var to override data directory (for persistent volumes)
	if envData := strings.TrimSpace(os.Getenv("DATA_DIR")); envData != "" {
		cfg.DataDir = envData
		log.Printf("DATA_DIR override: %q", envData)
	} else {
		log.Printf("DATA_DIR not set, using default: %q", cfg.DataDir)
	}
	// Also check Railway's auto-set volume env
	if volPath := strings.TrimSpace(os.Getenv("RAILWAY_VOLUME_MOUNT_PATH")); volPath != "" {
		log.Printf("RAILWAY_VOLUME_MOUNT_PATH: %q", volPath)
		if cfg.DataDir != volPath && strings.TrimSpace(os.Getenv("DATA_DIR")) == "" {
			cfg.DataDir = volPath
			log.Printf("Auto-using Railway volume path: %q", volPath)
		}
	}
	// Verify the data dir exists and log what's in it
	if entries, err := os.ReadDir(cfg.DataDir); err == nil {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		log.Printf("DataDir %q contents: %v", cfg.DataDir, names)
	} else {
		log.Printf("DataDir %q does not exist yet (will be created): %v", cfg.DataDir, err)
	}

	// Open database
	db, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	// Initialize providers
	providers := initProviders(cfg)

	// Initialize auth system (users, API keys, provider keys)
	authStore, err := auth.NewStore(db.Conn())
	if err != nil {
		log.Fatalf("auth store: %v", err)
	}
	providerFactory := auth.NewProviderFactory(authStore, providers)

	// Initialize shared components
	counter := tracker.NewSpendCounter()
	broadcaster := dashboard.NewBroadcaster()

	// Build the send handler (innermost — actually calls the provider)
	sendHandler := makeSendHandler(providers, providerFactory)

	// Create middleware toggle registry (allows runtime enable/disable via API)
	toggleReg := toggle.New()
	toggle.Global = toggleReg

	// Build middleware chain based on product features
	middlewares := buildMiddlewares(toggleReg, pc, cfg, db, counter, broadcaster, providers)

	// License enforcement — first in chain (prepend so it runs before everything)
	lic := license.FromEnv()
	licEnforcer := license.NewEnforcer(lic)
	middlewares = append([]proxy.Middleware{licEnforcer.Middleware()}, middlewares...)

	// Compose the handler
	handler := proxy.Chain(sendHandler, middlewares...)

	// OTEL export middleware (if configured via STOCKYARD_OTEL_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT)
	otelCfg := LoadOTELConfig()
	otelExp := NewOTELExporter(otelCfg)
	if otelExp != nil {
		handler = OTELMiddleware(otelExp)(handler)
	}

	// Wrap with app hooks (Observe traces + Trust audit) if apps are configured
	if len(pc.Apps) > 0 {
		handler = appHooksMiddleware(db.Conn())(handler)
	}

	// Build streaming pre-flight checks (so streaming requests also get
	// rate limit and cap enforcement instead of bypassing middleware)
	preFlight := buildPreFlight(pc, cfg, counter)

	// Create and configure the server
	var embedCache proxy.EmbeddingCacheProcessor
	if pc.Features.EmbedCache && cfg.EmbedCache.Enabled {
		embedCache = features.NewEmbedCache(cfg.EmbedCache)
		log.Printf("[embedcache] enabled: max_entries=%d ttl=%s", cfg.EmbedCache.MaxEntries, cfg.EmbedCache.TTL.Duration)
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Port:             cfg.Port,
		ProductName:      pc.Name,
		Handler:          handler,
		Providers:        providers,
		PreFlight:        preFlight,
		EmbedCache:       embedCache,
		ProviderResolver: providerFactory.ResolveProvider,
	})

	// Register dashboard, SSE, and management API
	dashboard.Register(srv.Mux(), pc.Product)
	broadcaster.RegisterSSE(srv.Mux())

	// Playground share endpoints
	registerPlaygroundRoutes(srv.Mux(), db.Conn())

	// Webhook manager (alerts, cost thresholds, trust violations → Slack, HTTP, etc.)
	webhookMgr := NewWebhookManager(db.Conn())
	RegisterWebhookRoutes(srv.Mux(), webhookMgr)

	// Status collector (real-time metrics for /api/status)
	statusCollector := NewStatusCollector()
	RegisterStatusRoutes(srv.Mux(), statusCollector, db.Conn(), pc.Version)

	// Config export/import/diff
	RegisterConfigRoutes(srv.Mux(), db.Conn())

	mgmtAPI := api.New(db, counter, pc.Product)
	mgmtAPI.SetHandler(handler) // Enable replay functionality
	mgmtAPI.Register(srv.Mux())

	// Register 6 flagship apps (if configured)
	if len(pc.Apps) > 0 {
		registry := platform.NewRegistry()
		for _, app := range pc.Apps {
			registry.Register(app)
		}
		// Run app-specific migrations on the shared database
		if err := registry.MigrateAll(db.Conn()); err != nil {
			log.Printf("app migrations: %v", err)
		}

		// Seed proxy modules from feature flags and providers
		seedProxyModules(db.Conn(), pc)
		seedProxyProviders(db.Conn(), providers)
		seedExchangePacks(db.Conn())
		seedForgeData(db.Conn())
		seedTrustData(db.Conn())

		// Seed toggle registry from proxy_modules table
		toggleReg.SeedFromDB(db.Conn())

		// Wire toggle registry to proxy app (if present)
		// First pass: wire basic dependencies
		for _, app := range pc.Apps {
			if setter, ok := app.(interface{ SetToggleRegistry(*toggle.Registry) }); ok {
				setter.SetToggleRegistry(toggleReg)
			}
			// Wire proxy port into forge app for workflow executor
			if setter, ok := app.(interface{ SetProxyPort(int) }); ok {
				setter.SetProxyPort(cfg.Port)
			}
			// Wire broadcaster into apps that subscribe to live events (observe, trust)
			if setter, ok := app.(interface{ SetBroadcaster(any) }); ok {
				setter.SetBroadcaster(broadcaster)
			}
		}

		// Second pass: extract trust auditor and wire to all apps + middleware
		var audit func(string, string, string, string, any)
		for _, app := range pc.Apps {
			if a, ok := app.(interface {
				Auditor() func(string, string, string, string, any)
			}); ok {
				audit = a.Auditor()
				break
			}
		}
		if audit != nil {
			// Wire auditor to apps that want it
			for _, app := range pc.Apps {
				if setter, ok := app.(interface {
					SetAuditor(func(string, string, string, string, any))
				}); ok {
					setter.SetAuditor(audit)
				}
			}
			// Record system boot event
			go audit("system", "engine", "stockyard", "boot", map[string]any{
				"apps":        len(pc.Apps),
				"middlewares":  len(middlewares),
				"port":        cfg.Port,
			})
			log.Printf("  Audit:     trust auditor wired to apps")
		}

		// Third pass: extract safety reporter from observe and wire to safety middlewares
		for _, app := range pc.Apps {
			if a, ok := app.(interface {
				SafetyReporter() func(string, string, string, string, string, string, string, string, any)
			}); ok {
				reporter := a.SafetyReporter()
				features.SetSafetyReporter(reporter)
				log.Printf("  Safety:    observe safety reporter wired to middlewares")
				break
			}
		}

		// Wire trust auditor to features so trust_enforce uses serialized hash chain
		if audit != nil {
			features.SetAuditFunc(audit)
			log.Printf("  Audit:     trust auditor wired to middlewares (hash chain safe)")
		}

		// Mount all app routes on the shared mux
		registry.RegisterAllRoutes(srv.Mux())

		// /api/apps — list all registered apps
		srv.Mux().HandleFunc("GET /api/apps", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"apps":  registry.AppList(),
				"count": len(registry.Apps()),
			})
		})
		log.Printf("  Apps:      %d registered (/api/apps)", len(pc.Apps))
	}

	// Mount sy-api billing/licensing/cloud/exchange routes (if enabled)
	if pc.EnableAPIServer {
		mountAPIServer(srv.Mux(), cfg.DataDir, authStore)
	}

	// Register auth API routes (user management, key management, provider keys)
	authAPI := auth.NewAPI(authStore)
	authAPI.SetLicenseEnforcer(licEnforcer)
	authAPI.Register(srv.Mux())

	// Wrap with self-service auth (/api/auth/me/* uses API key, not admin key)
	srv.WrapHandler(auth.SelfServiceAuthMiddleware(authStore))

	// Wrap with proxy auth (authenticates /v1/* requests with sk-sy- keys)
	proxyAuthMode := auth.GetProxyAuthMode()
	srv.WrapHandler(auth.ProxyAuthMiddleware(authStore, proxyAuthMode))

	// Wrap with auto-config (detects raw provider keys, creates ephemeral providers)
	srv.WrapHandler(auth.AutoConfigMiddleware(authStore, providerFactory))

	// Wrap with admin auth (reads STOCKYARD_ADMIN_KEY env var)
	srv.WrapHandler(adminAuthMiddleware)

	// Register marketing website (/, /docs/, /pricing/, etc.)
	site.Register(srv.Mux())

	// License status endpoint (uses same enforcer as middleware for accurate counts)
	srv.Mux().HandleFunc("GET /api/license", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(licEnforcer.Stats())
	})
	log.Printf("  License:   tier=%s valid=%v", licEnforcer.Tier(), lic.Valid)

	// OpenAPI spec
	srv.Mux().HandleFunc("GET /api/openapi.json", apiserver.HandleOpenAPI)
	log.Printf("  OpenAPI:   http://localhost:%d/api/openapi.json", cfg.Port)

	// Seed demo data if database is empty (populates traces, costs, experiments)
	db.SeedDemoData(pc.Product)

	// Start data retention cleanup loop
	db.StartCleanupLoop(cfg.Logging.RetentionDays, 0)

	// Start spend flusher (writes in-memory counters to SQLite every 5s)
	flushCtx, flushCancel := context.WithCancel(context.Background())
	flusher := tracker.NewFlusher(counter, db, 5*time.Second)
	go flusher.Start(flushCtx)

	// Start alert evaluator (checks alert rules every 60s, delivers webhooks)
	alertEval := observe.NewAlertEvaluator(db.Conn())
	go alertEval.Start(flushCtx)

	// Register branded 404 handler as catch-all (lowest priority pattern)
	notFound := site.NotFoundHandler()
	srv.Mux().HandleFunc("/{path...}", func(w http.ResponseWriter, r *http.Request) {
		notFound(w, r)
	})

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	log.Printf("══════════════════════════════════════")
	log.Printf("  %s is running", pc.Name)
	log.Printf("  Proxy:     http://localhost:%d/v1", cfg.Port)
	log.Printf("  Dashboard: http://localhost:%d/ui", cfg.Port)
	log.Printf("  API:       http://localhost:%d/api", cfg.Port)
	log.Printf("  Auth:      http://localhost:%d/api/auth (signup: POST /api/auth/signup)", cfg.Port)
	if proxyAuthMode == auth.ProxyAuthRequired {
		log.Printf("  🔒 Proxy auth: REQUIRED (set STOCKYARD_REQUIRE_AUTH=false to disable)")
	} else {
		log.Printf("  🔓 Proxy auth: open (set STOCKYARD_REQUIRE_AUTH=true to require API keys)")
	}
	if pc.EnableAPIServer {
		log.Printf("  Billing:   http://localhost:%d/api/checkout", cfg.Port)
		log.Printf("  Cloud:     http://localhost:%d/api/cloud/signup", cfg.Port)
		log.Printf("  Exchange:  http://localhost:%d/api/exchange", cfg.Port)
	}
	log.Printf("══════════════════════════════════════")

	<-ctx.Done()
	log.Println("shutting down...")
	flushCancel()
	if otelExp != nil {
		otelExp.Close()
	}
	srv.Shutdown(context.Background())
	db.Close()
}

// makeSendHandler creates the innermost handler that sends to the resolved provider.
func makeSendHandler(providers map[string]provider.Provider, factory *auth.ProviderFactory) proxy.Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		name := req.Provider
		if name == "" {
			name = provider.ProviderForModel(req.Model)
		}

		// Try auto-configured ephemeral provider (from raw API key in Authorization header)
		if autoP, autoName := auth.AutoProviderFromContext(ctx); autoP != nil {
			if name == "" || name == autoName {
				resp, err := autoP.Send(ctx, req)
				if err != nil {
					return nil, err
				}
				if resp.Provider == "" {
					resp.Provider = autoName
				}
				return resp, nil
			}
		}

		// Try user-specific provider (via factory)
		if factory != nil {
			if p, err := factory.ResolveProvider(ctx, name); err == nil && p != nil {
				resp, err := p.Send(ctx, req)
				if err != nil {
					return nil, err
				}
				if resp.Provider == "" {
					resp.Provider = name
				}
				return resp, nil
			}
		}

		// Fall back to global providers
		p, ok := providers[name]
		if !ok {
			// Try the first available provider as fallback
			for n, prov := range providers {
				p = prov
				name = n
				break
			}
			if p == nil {
				return nil, &providerError{msg: "no providers configured. Set OPENAI_API_KEY or other provider keys."}
			}
		}
		resp, err := p.Send(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp.Provider == "" {
			resp.Provider = name
		}
		return resp, nil
	}
}

// buildMiddlewares constructs the middleware chain based on product features.
// Applied in order: outermost first → innermost last.
// Chain reverses them, so the first middleware listed runs first.
func buildMiddlewares(reg *toggle.Registry, 
	pc ProductConfig,
	cfg *config.Config,
	db *storage.DB,
	counter *tracker.SpendCounter,
	broadcaster *dashboard.Broadcaster,
	providers map[string]provider.Provider,
) []proxy.Middleware {
	var mw []proxy.Middleware

	add := func(name string, m proxy.Middleware) {
		mw = append(mw, toggle.Wrap(name, reg, m))
	}

	// IPFence — block unauthorized IPs before any processing (outermost)
	if pc.Features.IPFence && cfg.IPFence.Enabled {
		fence := features.NewIPFence(cfg.IPFence)
		add("ipfence", features.IPFenceMiddleware(fence))
		log.Printf("ipfence: mode=%s action=%s allowlist=%d denylist=%d trust_proxy=%v",
			cfg.IPFence.Mode, cfg.IPFence.Action, len(cfg.IPFence.Allowlist), len(cfg.IPFence.Denylist), cfg.IPFence.TrustProxy)
	}

	// Rate limiting (outermost — reject before any work)
	if pc.Features.RateLimiting && cfg.RateLimit.Enabled {
		limiter := features.NewRateLimiter(features.RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: cfg.RateLimit.Default.RequestsPerMinute,
			RequestsPerHour:   cfg.RateLimit.Default.RequestsPerHour,
			Burst:             cfg.RateLimit.Default.Burst,
			PerIP:             cfg.RateLimit.PerIP,
			PerUser:           cfg.RateLimit.PerUser,
		})
		add("ratelimit", features.RateLimitMiddleware(limiter))
	}

	// KeyPool — rotate API keys before anything else touches the request
	if pc.Features.KeyPool && cfg.KeyPool.Enabled {
		pool := features.NewKeyPool(cfg.KeyPool)
		if pool.KeyCount() > 0 {
			add("keypool", features.KeyPoolMiddleware(pool))
			log.Printf("keypool: %d keys loaded, strategy=%s", pool.KeyCount(), cfg.KeyPool.Strategy)
		}
	}

	// TenantWall — per-tenant isolation (rate limits, budgets, model access)
	if pc.Features.TenantWall && cfg.TenantWall.Enabled {
		tw := features.NewTenantWall(cfg.TenantWall)
		add("tenantwall", features.TenantWallMiddleware(tw))
		log.Printf("tenantwall: require=%v window=%s default_max_req=%d default_max_spend=$%.2f tenants=%d",
			cfg.TenantWall.RequireTenant, cfg.TenantWall.WindowDuration.Duration,
			cfg.TenantWall.DefaultMaxRequests, cfg.TenantWall.DefaultMaxSpend, len(cfg.TenantWall.Tenants))
	}

	// PromptGuard — redact PII and detect injection before caching/logging
	if pc.Features.PromptGuard && cfg.PromptGuard.Enabled {
		guard := features.NewPromptGuard(cfg.PromptGuard)
		add("promptguard", features.PromptGuardMiddleware(guard, cfg.PromptGuard.Injection.Enabled))
		log.Printf("promptguard: mode=%s injection=%v", cfg.PromptGuard.PII.Mode, cfg.PromptGuard.Injection.Enabled)
	}

	// SecretScan — catch API keys and secrets in requests (before caching/logging)
	if pc.Features.SecretScan && cfg.SecretScan.Enabled {
		scanner := features.NewSecretScanner(cfg.SecretScan)
		add("secretscan", features.SecretScanMiddleware(scanner))
		log.Printf("secretscan: patterns=%d action=%s scan_input=%v scan_output=%v",
			scanner.PatternCount(), cfg.SecretScan.Action, cfg.SecretScan.ScanInput, cfg.SecretScan.ScanOutput)
	}

	// TokenTrim — context window optimization (before cache/send)
	if pc.Features.TokenTrim && cfg.TokenTrim.Enabled {
		trimmer := features.NewTokenTrimmer(cfg.TokenTrim)
		add("tokentrim", features.TokenTrimMiddleware(trimmer))
		log.Printf("tokentrim: strategy=%s safety_margin=%d", cfg.TokenTrim.DefaultStrat, cfg.TokenTrim.SafetyMargin)
	}

	// ContextPack — inject relevant context (before cache, after trim)
	if pc.Features.ContextPack && cfg.ContextPack.Enabled {
		packer := features.NewContextPacker(cfg.ContextPack)
		add("contextpack", features.ContextPackMiddleware(packer))
		log.Printf("contextpack: %d sources configured", len(cfg.ContextPack.Sources))
	}

	// ChatMem — inject conversation memory (after context, before prompt template)
	if pc.Features.ChatMem && cfg.ChatMem.Enabled {
		mem := features.NewChatMem(cfg.ChatMem)
		add("chatmem", features.ChatMemMiddleware(mem))
		log.Printf("chatmem: strategy=%s max_messages=%d inject=%v ttl=%s",
			cfg.ChatMem.Strategy, cfg.ChatMem.MaxMessages, cfg.ChatMem.InjectMemory, cfg.ChatMem.SessionTTL.Duration)
	}

	// PromptPad — template management and A/B testing (before cache)
	if pc.Features.PromptPad && cfg.PromptPad.Enabled {
		pad := features.NewPromptPad(cfg.PromptPad)
		add("promptpad", features.PromptPadMiddleware(pad))
		log.Printf("promptpad: %d templates loaded", len(cfg.PromptPad.Templates))
	}

	// IdleKill — kill runaway requests (timeout watchdog, before cache/provider)
	if pc.Features.IdleKill && cfg.IdleKill.Enabled {
		ik := features.NewIdleKill(cfg.IdleKill)
		add("idlekill", features.IdleKillMiddleware(ik))
		log.Printf("idlekill: max_duration=%s max_tokens=%d max_cost=$%.2f loop=%v",
			cfg.IdleKill.MaxDuration.Duration, cfg.IdleKill.MaxTokensPerRequest,
			cfg.IdleKill.MaxCostPerRequest, cfg.IdleKill.LoopDetection)
	}

	// Cache (before spending/logging — cache hits skip the provider)
	if pc.Features.Cache && cfg.Cache.Enabled {
		cache := features.NewCache(features.CacheConfig{
			Enabled:    true,
			Strategy:   cfg.Cache.Strategy,
			TTL:        cfg.Cache.TTL.Duration,
			MaxEntries: cfg.Cache.MaxEntries,
		})
		add("cache", features.CacheMiddleware(cache))
	}

	// Logging (captures everything including cache hits)
	if pc.Features.RequestLogging {
		bodySize := cfg.Logging.MaxBodySize
		if bodySize == 0 {
			bodySize = 50000
		}
		add("logging", features.LoggingMiddleware(features.LoggingConfig{
			StoreBodies: pc.Features.FullBodyLog || cfg.Logging.StoreBodies,
			MaxBodySize: bodySize,
			DB:          db,
			Broadcaster: broadcaster,
		}))
	}

	// Spend tracking
	if pc.Features.SpendTracking {
		caps := buildCaps(cfg)
		var alerter *features.Alerter
		if pc.Features.Alerts {
			alerter = buildAlerter(cfg)
		}
		add("spend", features.SpendMiddleware(features.SpendConfig{
			Counter:     counter,
			Alerter:     alerter,
			Caps:        caps,
			Broadcaster: broadcaster,
		}))
	}

	// Cap enforcement (pre-request check — blocks BEFORE sending)
	if pc.Features.SpendCaps {
		caps := buildCaps(cfg)
		add("caps", features.CapsMiddleware(caps, counter))
	}

	// UsagePulse — multi-dimensional metering (after caps, before routing)
	if pc.Features.UsagePulse && cfg.UsagePulse.Enabled {
		pulse := features.NewUsagePulse(cfg.UsagePulse)
		add("usagepulse", features.UsagePulseMiddleware(pulse))
		log.Printf("usagepulse: dimensions=%v", cfg.UsagePulse.Dimensions)
	}

	// ModelSwitch — smart routing (before failover, replaces model on request)
	if pc.Features.ModelSwitch && cfg.ModelSwitch.Enabled {
		router := features.NewModelRouter(cfg.ModelSwitch)
		add("modelswitch", features.ModelSwitchMiddleware(router, cfg.ModelSwitch.Default))
		log.Printf("modelswitch: %d rules loaded", len(cfg.ModelSwitch.Rules))
	}

	// MultiCall — multi-model consensus (fans out to multiple models)
	if pc.Features.MultiCall && cfg.MultiCall.Enabled {
		mc := features.NewMultiCaller(cfg.MultiCall)
		add("multicall", features.MultiCallMiddleware(mc, providers))
		log.Printf("multicall: %d routes configured", len(cfg.MultiCall.Routes))
	}

	// AnthroFit — deep Anthropic compatibility (before failover/routing)
	if pc.Features.AnthroFit && cfg.AnthroFit.Enabled {
		af := features.NewAnthroFit(cfg.AnthroFit)
		add("anthrofit", features.AnthroFitMiddleware(af))
		log.Printf("anthrofit: system_prompt=%s tools=%v stream_norm=%v max_tokens_default=%d",
			cfg.AnthroFit.SystemPromptMode, cfg.AnthroFit.ToolTranslation, cfg.AnthroFit.StreamNormalize, cfg.AnthroFit.MaxTokensDefault)
	}

	// MockLLM — deterministic fixture responses for testing/CI (intercepts before provider)
	if pc.Features.MockLLM && cfg.MockLLM.Enabled {
		mock := features.NewMockLLM(cfg.MockLLM)
		add("mockllm", features.MockLLMMiddleware(mock))
		log.Printf("mockllm: %d fixtures, passthrough=%v", len(cfg.MockLLM.Fixtures), cfg.MockLLM.Passthrough)
	}

	// Failover routing
	if pc.Features.Failover && cfg.Failover.Enabled {
		router := features.NewFailoverRouter(features.FailoverConfig{
			Enabled:          true,
			Strategy:         cfg.Failover.Strategy,
			Providers:        cfg.Failover.Providers,
			FailureThreshold: cfg.Failover.CircuitBreaker.FailureThreshold,
			RecoveryTimeout:  cfg.Failover.CircuitBreaker.RecoveryTimeout.Duration,
		})
		for name, p := range providers {
			prov := p
			router.RegisterSender(name, func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
				return prov.Send(ctx, req)
			})
		}
		add("failover", features.FailoverMiddleware(router))
	}

	// EvalGate — quality scoring + auto-retry (after provider sends, before final return)
	if pc.Features.EvalGate && cfg.EvalGate.Enabled {
		gate := features.NewEvalGate(cfg.EvalGate)
		add("evalgate", features.EvalGateMiddleware(gate))
		log.Printf("evalgate: %d validators, retry_budget=%d", len(cfg.EvalGate.Validators), cfg.EvalGate.RetryBudget)
	}

	// ToxicFilter — content moderation on outputs (after eval, before logging)
	if pc.Features.ToxicFilter && cfg.ToxicFilter.Enabled {
		filter := features.NewToxicFilter(cfg.ToxicFilter)
		add("toxicfilter", features.ToxicFilterMiddleware(filter))
		log.Printf("toxicfilter: action=%s scan_input=%v scan_output=%v categories=%d",
			cfg.ToxicFilter.Action, cfg.ToxicFilter.ScanInput, cfg.ToxicFilter.ScanOutput, len(cfg.ToxicFilter.Categories))
	}

	// Trust Policy Enforcement — checks requests/responses against trust_policies table
	// Always enabled when apps include Trust (policies are loaded from DB)
	if db != nil {
		enforcer := features.NewTrustEnforcer(db.Conn())
		add("trust_enforce", enforcer.Middleware())
		log.Printf("trust_enforce: policy enforcement enabled")
	}

	// TraceLink — distributed tracing (wraps request lifecycle)
	if pc.Features.TraceLink && cfg.TraceLink.Enabled {
		tracer := features.NewTraceLinker(cfg.TraceLink)
		add("tracelink", features.TraceLinkMiddleware(tracer))
		log.Printf("tracelink: service=%s sample_rate=%.2f w3c=%v", cfg.TraceLink.ServiceName, cfg.TraceLink.SampleRate, cfg.TraceLink.PropagateW3C)
	}

	// StreamSnap — SSE response capture and metrics
	if pc.Features.StreamSnap && cfg.StreamSnap.Enabled {
		snapper := features.NewStreamSnapper(cfg.StreamSnap)
		add("streamsnap", features.StreamSnapMiddleware(snapper))
		log.Printf("streamsnap: capture enabled, metrics ttft=%v tps=%v", cfg.StreamSnap.Metrics.TTFT, cfg.StreamSnap.Metrics.TPS)
	}

	// LLMTap — analytics recording (outermost post-request, captures everything)
	if pc.Features.LLMTap && cfg.LLMTap.Enabled {
		tap := features.NewLLMTap(cfg.LLMTap)
		add("llmtap", features.LLMTapMiddleware(tap))
		log.Printf("llmtap: analytics enabled, percentiles=%v", cfg.LLMTap.Percentiles)
	}

	// ComplianceLog — immutable audit trail with hash chains (after all processing)
	if pc.Features.ComplianceLog && cfg.ComplianceLog.Enabled {
		cl := features.NewComplianceLogger(cfg.ComplianceLog)
		add("compliancelog", features.ComplianceLogMiddleware(cl))
		log.Printf("compliancelog: hash=%s retention=%dd exports=%v",
			cfg.ComplianceLog.HashAlgorithm, cfg.ComplianceLog.RetentionDays, cfg.ComplianceLog.ExportFormats)
	}

	// AlertPulse — alerting engine for error rates, latency, cost spikes
	if pc.Features.AlertPulse && cfg.AlertPulse.Enabled {
		ap := features.NewAlertPulse(cfg.AlertPulse)
		add("alertpulse", features.AlertPulseMiddleware(ap))
		log.Printf("alertpulse: %d rules, window=%s cooldown=%s",
			len(cfg.AlertPulse.Rules), cfg.AlertPulse.WindowDuration.Duration, cfg.AlertPulse.Cooldown.Duration)
	}

	// RetryPilot — intelligent retry with circuit breaking (replaces basic retry)
	if pc.Features.RetryPilot && cfg.RetryPilot.Enabled {
		pilot := features.NewRetryPilot(cfg.RetryPilot)
		add("retrypilot", features.RetryPilotMiddleware(pilot))
		log.Printf("retrypilot: max_retries=%d jitter=%s downgrade=%v", cfg.RetryPilot.MaxRetries, cfg.RetryPilot.Jitter, cfg.RetryPilot.Downgrade.Enabled)
	} else {
		// Basic retry (innermost middleware, closest to the provider send)
		retries := 2
		for _, p := range cfg.Providers {
			if p.MaxRetries > 0 {
				retries = p.MaxRetries
				break
			}
		}
		add("retry", features.RetryMiddleware(retries))
	}

	// Note: BatchQueue operates as an API endpoint (POST /api/batch) rather than inline middleware.
	// It's registered separately in engine.Boot() when enabled.

	// ── Phase 3 P2 middleware ──

	// PromptSlim — compress prompts (before cache/send)
	if pc.Features.PromptSlim && cfg.PromptSlim.Enabled {
		slim := features.NewPromptSlim(cfg.PromptSlim)
		add("promptslim", features.PromptSlimMiddleware(slim))
		log.Printf("promptslim: aggressiveness=%.1f", cfg.PromptSlim.Aggressiveness)
	}

	// PromptLint — check prompt quality (before cache/send)
	if pc.Features.PromptLint && cfg.PromptLint.Enabled {
		lint := features.NewPromptLint(cfg.PromptLint)
		add("promptlint", features.PromptLintMiddleware(lint))
		log.Printf("promptlint: block_on_fail=%v", cfg.PromptLint.BlockOnFail)
	}

	// ApprovalGate — prompt change approval (before send)
	if pc.Features.ApprovalGate && cfg.ApprovalGate.Enabled {
		gate := features.NewApprovalGate(cfg.ApprovalGate)
		add("approvalgate", features.ApprovalGateMiddleware(gate))
		log.Printf("approvalgate: approvers=%d", len(cfg.ApprovalGate.Approvers))
	}

	// ContextWindow — visual context window debugger (before send)
	if pc.Features.ContextWindow && cfg.ContextWindow.Enabled {
		cw := features.NewContextWindow(cfg.ContextWindow)
		add("contextwindow", features.ContextWindowMiddleware(cw))
		log.Printf("contextwindow: enabled")
	}

	// TierDrop — auto-downgrade models when near budget (before routing)
	if pc.Features.TierDrop && cfg.TierDrop.Enabled {
		td := features.NewTierDrop(cfg.TierDrop)
		add("tierdrop", features.TierDropMiddleware(td))
		log.Printf("tierdrop: %d tiers configured", len(cfg.TierDrop.Tiers))
	}

	// ABRouter — A/B testing experiments (before routing)
	if pc.Features.ABRouter && cfg.ABRouter.Enabled {
		ab := features.NewABRouter(cfg.ABRouter)
		add("abrouter", features.ABRouterMiddleware(ab))
		log.Printf("abrouter: %d experiments", len(cfg.ABRouter.Experiments))
	}

	// LangBridge — cross-language translation (before send)
	if pc.Features.LangBridge && cfg.LangBridge.Enabled {
		lb := features.NewLangBridge(cfg.LangBridge)
		add("langbridge", features.LangBridgeMiddleware(lb))
		log.Printf("langbridge: target=%s", cfg.LangBridge.TargetLang)
	}

	// GeminiShim — Gemini compatibility (before failover)
	if pc.Features.GeminiShim && cfg.GeminiShim.Enabled {
		gs := features.NewGeminiShim(cfg.GeminiShim)
		add("geminishim", features.GeminiShimMiddleware(gs))
		log.Printf("geminishim: auto_retry_safety=%v normalize=%v", cfg.GeminiShim.AutoRetrySafety, cfg.GeminiShim.NormalizeTokens)
	}

	// LocalSync — local/cloud model blending (before failover)
	if pc.Features.LocalSync && cfg.LocalSync.Enabled {
		ls := features.NewLocalSync(cfg.LocalSync)
		add("localsync", features.LocalSyncMiddleware(ls))
		log.Printf("localsync: local=%s fallback=%v", cfg.LocalSync.LocalEndpoint, cfg.LocalSync.FallbackToCloud)
	}

	// RegionRoute — data residency routing (before failover)
	if pc.Features.RegionRoute && cfg.RegionRoute.Enabled {
		rr := features.NewRegionRoute(cfg.RegionRoute)
		add("regionroute", features.RegionRouteMiddleware(rr))
		log.Printf("regionroute: %d routes", len(cfg.RegionRoute.Routes))
	}

	// AgentGuard — agent session safety rails (wraps request lifecycle)
	if pc.Features.AgentGuard && cfg.AgentGuard.Enabled {
		ag := features.NewAgentGuard(cfg.AgentGuard)
		add("agentguard", features.AgentGuardMiddleware(ag))
		log.Printf("agentguard: max_calls=%d max_cost=$%.2f max_duration=%s",
			cfg.AgentGuard.MaxCalls, cfg.AgentGuard.MaxCost, cfg.AgentGuard.MaxDuration.Duration)
	}

	// DevProxy — developer debugging (wraps request lifecycle)
	if pc.Features.DevProxy && cfg.DevProxy.Enabled {
		dp := features.NewDevProxy(cfg.DevProxy)
		add("devproxy", features.DevProxyMiddleware(dp))
		log.Printf("devproxy: log_headers=%v log_bodies=%v", cfg.DevProxy.LogHeaders, cfg.DevProxy.LogBodies)
	}

	// DriftWatch — model drift detection (post-response)
	if pc.Features.DriftWatch && cfg.DriftWatch.Enabled {
		dw := features.NewDriftWatch(cfg.DriftWatch)
		add("driftwatch", features.DriftWatchMiddleware(dw))
		log.Printf("driftwatch: threshold=%.0f%%", cfg.DriftWatch.DriftThreshold)
	}

	// CodeFence — code validation (post-response)
	if pc.Features.CodeFence && cfg.CodeFence.Enabled {
		cf := features.NewCodeFence(cfg.CodeFence)
		add("codefence", features.CodeFenceMiddleware(cf))
		log.Printf("codefence: patterns=%d action=%s", len(cfg.CodeFence.ForbiddenPatterns), cfg.CodeFence.Action)
	}

	// HalluciCheck — hallucination detection (post-response)
	if pc.Features.HalluciCheck && cfg.HalluciCheck.Enabled {
		hc := features.NewHalluciCheck(cfg.HalluciCheck)
		add("hallucicheck", features.HalluciCheckMiddleware(hc))
		log.Printf("hallucicheck: urls=%v emails=%v", cfg.HalluciCheck.CheckURLs, cfg.HalluciCheck.CheckEmails)
	}

	// GuardRail — topic fencing (post-response)
	if pc.Features.GuardRail && cfg.GuardRail.Enabled {
		gr := features.NewGuardRail(cfg.GuardRail)
		add("guardrail", features.GuardRailMiddleware(gr))
		log.Printf("guardrail: allowed=%d denied=%d", len(cfg.GuardRail.AllowedTopics), len(cfg.GuardRail.DeniedTopics))
	}

	// AgeGate — child safety filtering (post-response)
	if pc.Features.AgeGate && cfg.AgeGate.Enabled {
		ag := features.NewAgeGate(cfg.AgeGate)
		add("agegate", features.AgeGateMiddleware(ag))
		log.Printf("agegate: tier=%s", cfg.AgeGate.Tier)
	}

	// OutputCap — output length capping (post-response)
	if pc.Features.OutputCap && cfg.OutputCap.Enabled {
		oc := features.NewOutputCap(cfg.OutputCap)
		add("outputcap", features.OutputCapMiddleware(oc))
		log.Printf("outputcap: max_chars=%d", cfg.OutputCap.MaxChars)
	}

	// VoiceBridge — voice pipeline optimization (post-response)
	if pc.Features.VoiceBridge && cfg.VoiceBridge.Enabled {
		vb := features.NewVoiceBridge(cfg.VoiceBridge)
		add("voicebridge", features.VoiceBridgeMiddleware(vb))
		log.Printf("voicebridge: max_length=%d", cfg.VoiceBridge.MaxLength)
	}

	// ImageProxy — image generation proxy (passthrough)
	if pc.Features.ImageProxy && cfg.ImageProxy.Enabled {
		ip := features.NewImageProxy(cfg.ImageProxy)
		add("imageproxy", features.ImageProxyMiddleware(ip))
		log.Printf("imageproxy: cache=%v", cfg.ImageProxy.CacheEnabled)
	}

	// FeedbackLoop — user feedback collection (passthrough)
	if pc.Features.FeedbackLoop && cfg.FeedbackLoop.Enabled {
		fl := features.NewFeedbackLoop(cfg.FeedbackLoop)
		add("feedbackloop", features.FeedbackLoopMiddleware(fl))
		log.Printf("feedbackloop: endpoint=%s", cfg.FeedbackLoop.Endpoint)
	}

	// ── Phase 3 P3 middleware ──

	// ChainForge — multi-step LLM workflows
	if pc.Features.ChainForge && cfg.ChainForge.Enabled {
		cf := features.NewChainForge(cfg.ChainForge)
		add("chainforge", features.ChainForgeMiddleware(cf))
		log.Printf("chainforge: %d pipelines", len(cfg.ChainForge.Pipelines))
	}

	// CronLLM — scheduled LLM tasks
	if pc.Features.CronLLM && cfg.CronLLM.Enabled {
		cl := features.NewCronLLM(cfg.CronLLM)
		add("cronllm", features.CronLLMMiddleware(cl))
		log.Printf("cronllm: %d jobs configured", len(cfg.CronLLM.Jobs))
	}

	// WebhookRelay — webhook-to-LLM relay
	if pc.Features.WebhookRelay && cfg.WebhookRelay.Enabled {
		wr := features.NewWebhookRelay(cfg.WebhookRelay)
		add("webhookrelay", features.WebhookRelayMiddleware(wr))
		log.Printf("webhookrelay: %d triggers", len(cfg.WebhookRelay.Triggers))
	}

	// BillSync — per-customer invoicing
	if pc.Features.BillSync && cfg.BillSync.Enabled {
		bs := features.NewBillSync(cfg.BillSync)
		add("billsync", features.BillSyncMiddleware(bs))
		log.Printf("billsync: markup=%.0f%% currency=%s", cfg.BillSync.MarkupPct, cfg.BillSync.Currency)
	}

	// WhiteLabel — custom branding (passthrough, brand applies to dashboard)
	if pc.Features.WhiteLabel && cfg.WhiteLabel.Enabled {
		wl := features.NewWhiteLabel(cfg.WhiteLabel)
		add("whitelabel", features.WhiteLabelMiddleware(wl))
		log.Printf("whitelabel: brand=%s", cfg.WhiteLabel.BrandName)
	}

	// TrainExport — training data collection
	if pc.Features.TrainExport && cfg.TrainExport.Enabled {
		te := features.NewTrainExport(cfg.TrainExport)
		add("trainexport", features.TrainExportMiddleware(te))
		log.Printf("trainexport: format=%s max_pairs=%d", cfg.TrainExport.Format, cfg.TrainExport.MaxPairs)
	}

	// SynthGen — synthetic data generation
	if pc.Features.SynthGen && cfg.SynthGen.Enabled {
		sg := features.NewSynthGen(cfg.SynthGen)
		add("synthgen", features.SynthGenMiddleware(sg))
		log.Printf("synthgen: batch_size=%d", cfg.SynthGen.BatchSize)
	}

	// DiffPrompt — prompt change detection
	if pc.Features.DiffPrompt && cfg.DiffPrompt.Enabled {
		dp := features.NewDiffPrompt(cfg.DiffPrompt)
		add("diffprompt", features.DiffPromptMiddleware(dp))
		log.Printf("diffprompt: enabled")
	}

	// LLMBench — model benchmarking
	if pc.Features.LLMBench && cfg.LLMBench.Enabled {
		lb := features.NewLLMBench(cfg.LLMBench)
		add("llmbench", features.LLMBenchMiddleware(lb))
		log.Printf("llmbench: enabled")
	}

	// MaskMode — demo mode with fake data
	if pc.Features.MaskMode && cfg.MaskMode.Enabled {
		mm := features.NewMaskMode(cfg.MaskMode)
		add("maskmode", features.MaskModeMiddleware(mm))
		log.Printf("maskmode: names=%v email=%v phone=%v", cfg.MaskMode.MaskNames, cfg.MaskMode.MaskEmail, cfg.MaskMode.MaskPhone)
	}

	// TokenMarket — dynamic budget reallocation
	if pc.Features.TokenMarket && cfg.TokenMarket.Enabled {
		tm := features.NewTokenMarket(cfg.TokenMarket)
		add("tokenmarket", features.TokenMarketMiddleware(tm))
		log.Printf("tokenmarket: %d pools", len(cfg.TokenMarket.Pools))
	}

	// LLMSync — config sync (passthrough)
	if pc.Features.LLMSync && cfg.LLMSync.Enabled {
		ls := features.NewLLMSync(cfg.LLMSync)
		add("llmsync", features.LLMSyncMiddleware(ls))
		log.Printf("llmsync: env=%s", cfg.LLMSync.Environment)
	}

	// ClusterMode — multi-instance coordination (passthrough)
	if pc.Features.ClusterMode && cfg.ClusterMode.Enabled {
		cm := features.NewClusterMode(cfg.ClusterMode)
		add("clustermode", features.ClusterModeMiddleware(cm))
		log.Printf("clustermode: node=%s peers=%d", cfg.ClusterMode.NodeID, len(cfg.ClusterMode.Peers))
	}

	// EncryptVault — encryption (passthrough)
	if pc.Features.EncryptVault && cfg.EncryptVault.Enabled {
		ev := features.NewEncryptVault(cfg.EncryptVault)
		add("encryptvault", features.EncryptVaultMiddleware(ev))
		log.Printf("encryptvault: active=%v", cfg.EncryptVault.Key != "")
	}

	// MirrorTest — shadow testing (needs providers for shadow calls)
	if pc.Features.MirrorTest && cfg.MirrorTest.Enabled {
		mt := features.NewMirrorTest(cfg.MirrorTest)
		add("mirrortest", features.MirrorTestMiddleware(mt, providers))
		log.Printf("mirrortest: shadow=%s sample=%.0f%%", cfg.MirrorTest.ShadowModel, cfg.MirrorTest.SampleRate*100)
	}

	// ── Phase 4 middleware ──
	mw = buildPhase4Middlewares(pc, cfg, providers, mw)

	return mw
}

// buildCaps extracts cap configuration from the config.
func buildCaps(cfg *config.Config) map[string]features.CapConfig {
	caps := make(map[string]features.CapConfig)
	for name, proj := range cfg.Projects {
		caps[name] = features.CapConfig{
			DailyCap:   proj.Caps.Daily,
			MonthlyCap: proj.Caps.Monthly,
		}
	}
	return caps
}

// buildAlerter creates an alerter from config.
func buildAlerter(cfg *config.Config) *features.Alerter {
	// Find the first project with a webhook configured
	for _, proj := range cfg.Projects {
		if proj.Alerts.Webhook != "" {
			return features.NewAlerter(features.AlertConfig{
				WebhookURL: proj.Alerts.Webhook,
				Thresholds: proj.Alerts.Thresholds,
			})
		}
	}
	// Return a no-op alerter (no webhook configured)
	return features.NewAlerter(features.AlertConfig{})
}

// initProviders creates provider instances from config and environment variables.
func initProviders(cfg *config.Config) map[string]provider.Provider {
	providers := make(map[string]provider.Provider)

	// Config-based providers (from stockyard.yaml)
	if p, ok := cfg.Providers["openai"]; ok && p.APIKey != "" && !isTemplate(p.APIKey) {
		providers["openai"] = provider.NewOpenAI(provider.ProviderConfig{
			APIKey: p.APIKey, BaseURL: p.BaseURL, Timeout: p.Timeout.Duration,
		})
	}
	if p, ok := cfg.Providers["anthropic"]; ok && p.APIKey != "" && !isTemplate(p.APIKey) {
		providers["anthropic"] = provider.NewAnthropic(provider.ProviderConfig{
			APIKey: p.APIKey, BaseURL: p.BaseURL, Timeout: p.Timeout.Duration,
		})
	}
	if p, ok := cfg.Providers["groq"]; ok && p.APIKey != "" && !isTemplate(p.APIKey) {
		providers["groq"] = provider.NewGroq(provider.ProviderConfig{
			APIKey: p.APIKey, BaseURL: p.BaseURL, Timeout: p.Timeout.Duration,
		})
	}
	if p, ok := cfg.Providers["gemini"]; ok && p.APIKey != "" && !isTemplate(p.APIKey) {
		providers["gemini"] = provider.NewGemini(provider.ProviderConfig{
			APIKey: p.APIKey, BaseURL: p.BaseURL, Timeout: p.Timeout.Duration,
		})
	}

	// Auto-detect providers from environment variables
	envProviders := map[string]struct {
		envKey  string
		factory func(provider.ProviderConfig) provider.Provider
	}{
		"openai":     {"OPENAI_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewOpenAI(c) }},
		"anthropic":  {"ANTHROPIC_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewAnthropic(c) }},
		"gemini":     {"GEMINI_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewGemini(c) }},
		"groq":       {"GROQ_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewGroq(c) }},
		"mistral":    {"MISTRAL_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewMistral(c) }},
		"together":   {"TOGETHER_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewTogether(c) }},
		"deepseek":   {"DEEPSEEK_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewDeepSeek(c) }},
		"fireworks":  {"FIREWORKS_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewFireworks(c) }},
		"perplexity": {"PERPLEXITY_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewPerplexity(c) }},
		"openrouter": {"OPENROUTER_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewOpenRouter(c) }},
		"xai":        {"XAI_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewXAI(c) }},
		"cohere":     {"COHERE_API_KEY", func(c provider.ProviderConfig) provider.Provider { return provider.NewCohere(c) }},
		"replicate":  {"REPLICATE_API_TOKEN", func(c provider.ProviderConfig) provider.Provider { return provider.NewReplicate(c) }},
	}

	for name, ep := range envProviders {
		if _, exists := providers[name]; exists {
			continue // Already configured
		}
		if key := os.Getenv(ep.envKey); key != "" {
			providers[name] = ep.factory(provider.ProviderConfig{
				APIKey:  key,
				Timeout: 60 * time.Second,
			})
			log.Printf("  Provider: %s (from %s)", name, ep.envKey)
		}
	}

	if len(providers) == 0 {
		log.Println("⚠️  No API keys configured. Set OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.")
	} else {
		names := make([]string, 0, len(providers))
		for n := range providers {
			names = append(names, n)
		}
		log.Printf("  Providers: %d (%s)", len(providers), strings.Join(names, ", "))
	}

	return providers
}

// isTemplate checks if a value is still an unresolved env var template.
func isTemplate(s string) bool {
	return len(s) > 3 && s[0] == '$' && s[1] == '{'
}

// buildPreFlight creates pre-flight checks for streaming requests.
// These mirror the middleware chain's rate limit and cap enforcement.
func buildPreFlight(pc ProductConfig, cfg *config.Config, counter *tracker.SpendCounter) proxy.StreamPreFlight {
	pf := proxy.StreamPreFlight{}

	// Rate limit check for streaming
	if pc.Features.RateLimiting && cfg.RateLimit.Enabled {
		limiter := features.NewRateLimiter(features.RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: cfg.RateLimit.Default.RequestsPerMinute,
			RequestsPerHour:   cfg.RateLimit.Default.RequestsPerHour,
			Burst:             cfg.RateLimit.Default.Burst,
			PerIP:             cfg.RateLimit.PerIP,
			PerUser:           cfg.RateLimit.PerUser,
		})
		pf.CheckRateLimit = func(req *provider.Request) error {
			key := req.Project
			if req.UserID != "" {
				key = req.UserID
			}
			if !limiter.Allow(key) {
				return fmt.Errorf("rate limit exceeded for %s", key)
			}
			return nil
		}
	}

	// Cap check for streaming
	if pc.Features.SpendCaps {
		caps := buildCaps(cfg)
		pf.CheckCaps = func(req *provider.Request) error {
			capCfg, ok := caps[req.Project]
			if !ok {
				capCfg = caps["default"]
			}
			spend := counter.Get(req.Project)
			if capCfg.DailyCap > 0 && spend.Today >= capCfg.DailyCap && !capCfg.SoftCap {
				return fmt.Errorf("daily cap exceeded: spent $%.4f of $%.2f cap", spend.Today, capCfg.DailyCap)
			}
			if capCfg.MonthlyCap > 0 && spend.Month >= capCfg.MonthlyCap && !capCfg.SoftCap {
				return fmt.Errorf("monthly cap exceeded: spent $%.4f of $%.2f cap", spend.Month, capCfg.MonthlyCap)
			}
			return nil
		}
	}

	// Failover chain for streaming
	if pc.Features.Failover && cfg.Failover.Enabled {
		pf.ResolveProvider = func(req *provider.Request) []string {
			return cfg.Failover.Providers
		}
	}

	// Post-stream spend tracking (so streaming requests count toward spend)
	if pc.Features.SpendTracking {
		pf.OnStreamComplete = func(req *provider.Request, providerName string, outputTokens int) {
			inputTokens := tracker.CountInputTokens(req.Model, req.Messages)
			cost := provider.CalculateCost(req.Model, inputTokens, outputTokens)
			counter.Add(req.Project, cost)
			log.Printf("stream spend: project=%s model=%s tokens_in=%d tokens_out=%d cost=$%.6f provider=%s",
				req.Project, req.Model, inputTokens, outputTokens, cost, providerName)
		}
	}

	return pf
}

// providerError is a simple error type for provider issues.
type providerError struct {
	msg string
}

func (e *providerError) Error() string { return e.msg }
