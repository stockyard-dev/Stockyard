// BootProxy starts the open-source proxy binary.
// This is the minimal boot path: proxy core, provider routing, and
// ~20 core middleware modules. No dashboard, no apps, no platform products.
package engine

import (
	"context"
	"database/sql"
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

	"github.com/stockyard-dev/stockyard/internal/auth"
	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/features"
	"github.com/stockyard-dev/stockyard/internal/mcp"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/slog"
	"github.com/stockyard-dev/stockyard/internal/storage"
	"github.com/stockyard-dev/stockyard/internal/toggle"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// ProxyConfig defines the OSS proxy configuration.
type ProxyConfig struct {
	Version string
}

// BootProxy starts the open-source Stockyard proxy.
func BootProxy(pc ProxyConfig) {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		v := pc.Version
		if v == "" {
			v = "dev"
		}
		fmt.Printf("stockyard-proxy %s\n", v)
		os.Exit(0)
	}

	if len(os.Args) > 1 && (os.Args[1] == "--health" || os.Args[1] == "health") {
		fmt.Println("ok")
		os.Exit(0)
	}

	log.SetFlags(log.Ltime | log.Lshortfile)
	slog.Init(slog.Config{
		Level:  os.Getenv("LOG_LEVEL"),
		Format: os.Getenv("LOG_FORMAT"),
	})

	configPath := ""
	if len(os.Args) > 1 && os.Args[1] != "serve" {
		configPath = os.Args[1]
	}
	cfg, err := config.LoadOrDefault(configPath, "stockyard-proxy")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}
	if envData := strings.TrimSpace(os.Getenv("DATA_DIR")); envData != "" {
		cfg.DataDir = envData
	}
	if volPath := strings.TrimSpace(os.Getenv("RAILWAY_VOLUME_MOUNT_PATH")); volPath != "" {
		if strings.TrimSpace(os.Getenv("DATA_DIR")) == "" {
			cfg.DataDir = volPath
		}
	}

	db, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	providers := initProviders(cfg)

	authStore, err := auth.NewStore(db.Conn())
	if err != nil {
		log.Fatalf("auth store: %v", err)
	}
	providerFactory := auth.NewProviderFactory(authStore, providers)

	counter := tracker.NewSpendCounter()

	sendHandler := makeSendHandler(providers, providerFactory)

	toggleReg := toggle.New()
	toggle.Global = toggleReg

	middlewares, failoverRouter, modelAliasState := buildProxyMiddlewares(toggleReg, cfg, db, counter, providers)
	handler := proxy.Chain(sendHandler, middlewares...)

	srv := proxy.NewServer(proxy.ServerConfig{
		Port:             cfg.Port,
		ProductName:      "Stockyard Proxy",
		Handler:          handler,
		Providers:        providers,
		ProviderResolver: providerFactory.ResolveProvider,
	})

	toggleReg.SeedFromDB(db.Conn())

	// Register proxy management API
	proxyMgmt := &proxyAppShim{conn: db.Conn(), toggle: toggleReg, failover: failoverRouter, modelAlias: modelAliasState}
	if err := proxyMgmt.Migrate(db.Conn()); err != nil {
		log.Printf("[proxy] migration error: %v", err)
	}
	proxyMgmt.RegisterRoutes(srv.Mux())

	// Auth API
	authAPI := auth.NewAPI(authStore)
	authAPI.Register(srv.Mux())

	// Version
	srv.Mux().HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		v := pc.Version
		if v == "" {
			v = "dev"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"version":   v,
			"product":   "stockyard-proxy",
			"edition":   "open-source",
			"providers": len(providers),
			"modules":   len(toggleReg.KnownModules()),
		})
	})

	// Health
	srv.Mux().HandleFunc("GET /api/platform/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"edition":   "open-source",
			"providers": len(providers),
			"modules":   len(toggleReg.KnownModules()),
		})
	})

	// Basic traces
	registerProxyTraceRoutes(srv.Mux(), db.Conn())

	// MCP server (SSE transport for Cursor, Windsurf, Cline, Aider)
	mcpServer := mcp.NewServer(handler)
	mcpServer.SetDB(db.Conn())
	mcpServer.SetToggle(toggleReg)
	mcpServer.Register(srv.Mux())

	// Auth wrappers
	proxyAuthMode := auth.GetProxyAuthMode()
	srv.WrapHandler(auth.ProxyAuthMiddleware(authStore, proxyAuthMode))
	srv.WrapHandler(auth.AutoConfigMiddleware(authStore, providerFactory))
	srv.WrapHandler(adminAuthMiddleware)
	srv.WrapHandler(gzipMiddleware)
	srv.WrapHandler(recoveryMiddleware)

	// 404
	srv.Mux().HandleFunc("/{path...}", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"message":"unknown endpoint","type":"invalid_request_error"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found","upgrade":"Full dashboard and apps at stockyard.dev"}`))
	})

	// Spend flusher
	flushCtx, flushCancel := context.WithCancel(context.Background())
	flusher := tracker.NewFlusher(counter, db, 10*time.Second)
	go flusher.Start(flushCtx)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	modCount := len(toggleReg.KnownModules())
	provCount := len(providers)
	log.Printf("")
	log.Printf("  Stockyard Proxy (Open Source)")
	log.Printf("  Proxy:      http://localhost:%d/v1", cfg.Port)
	log.Printf("  Modules:    %d active", modCount)
	log.Printf("  Providers:  %d configured", provCount)
	log.Printf("")
	log.Printf("  Quick start:")
	log.Printf("    export OPENAI_BASE_URL=http://localhost:%d/v1", cfg.Port)
	log.Printf("")
	if proxyAuthMode == auth.ProxyAuthRequired {
		log.Printf("  Proxy auth: REQUIRED")
	} else {
		log.Printf("  Proxy auth: open")
	}
	log.Printf("  Full platform: https://stockyard.dev")
	log.Printf("")

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	flushCancel()
}

// buildProxyMiddlewares builds the OSS middleware chain (core modules only).
func buildProxyMiddlewares(
	reg *toggle.Registry,
	cfg *config.Config,
	db *storage.DB,
	counter *tracker.SpendCounter,
	providers map[string]provider.Provider,
) ([]proxy.Middleware, *features.FailoverRouter, *features.ModelAliasState) {
	var mw []proxy.Middleware
	var failoverRouter *features.FailoverRouter

	add := func(name string, m proxy.Middleware) {
		mw = append(mw, toggle.Wrap(name, reg, m))
	}

	// IP fence
	if cfg.IPFence.Enabled {
		fence := features.NewIPFence(cfg.IPFence)
		add("ipfence", features.IPFenceMiddleware(fence))
	}

	// Rate limiting
	if cfg.RateLimit.Enabled {
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

	// Token trimming
	if cfg.TokenTrim.Enabled {
		trimmer := features.NewTokenTrimmer(cfg.TokenTrim)
		add("tokentrim", features.TokenTrimMiddleware(trimmer))
	}

	// Context window
	if cfg.ContextWindow.Enabled {
		cw := features.NewContextWindow(cfg.ContextWindow)
		add("contextwindow", features.ContextWindowMiddleware(cw))
	}

	// Model alias
	modelAliasState := features.NewModelAlias(cfg.ModelAlias)
	add("modelalias", features.ModelAliasMiddleware(modelAliasState))

	// Model switch
	if cfg.ModelSwitch.Enabled {
		router := features.NewModelRouter(cfg.ModelSwitch)
		add("modelswitch", features.ModelSwitchMiddleware(router, cfg.ModelSwitch.Default))
	}

	// AB router
	if cfg.ABRouter.Enabled {
		router := features.NewABRouter(cfg.ABRouter)
		add("abrouter", features.ABRouterMiddleware(router))
	}

	// Failover routing
	if cfg.Failover.Enabled {
		failoverRouter = features.NewFailoverRouter(features.FailoverConfig{
			Enabled:          true,
			Strategy:         cfg.Failover.Strategy,
			Providers:        cfg.Failover.Providers,
			FailureThreshold: cfg.Failover.CircuitBreaker.FailureThreshold,
			RecoveryTimeout:  cfg.Failover.CircuitBreaker.RecoveryTimeout.Duration,
		})
		for name, p := range providers {
			prov := p
			failoverRouter.RegisterSender(name, func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
				return prov.Send(ctx, req)
			})
		}
		add("failover", features.FailoverMiddleware(failoverRouter))
	}

	// Gemini shim
	if cfg.GeminiShim.Enabled {
		gs := features.NewGeminiShim(cfg.GeminiShim)
		add("geminishim", features.GeminiShimMiddleware(gs))
	}

	// Anthropic shim
	if cfg.AnthroFit.Enabled {
		af := features.NewAnthroFit(cfg.AnthroFit)
		add("anthrofit", features.AnthroFitMiddleware(af))
	}

	// Cache
	if cfg.Cache.Enabled {
		cache := features.NewCache(features.CacheConfig{
			Enabled:    true,
			Strategy:   cfg.Cache.Strategy,
			TTL:        cfg.Cache.TTL.Duration,
			MaxEntries: cfg.Cache.MaxEntries,
		})
		add("cache", features.CacheMiddleware(cache))
	}

	// Retry
	retries := 2
	for _, p := range cfg.Providers {
		if p.MaxRetries > 0 {
			retries = p.MaxRetries
			break
		}
	}
	add("retry", features.RetryMiddleware(retries))

	// Retry pilot
	if cfg.RetryPilot.Enabled {
		rp := features.NewRetryPilot(cfg.RetryPilot)
		add("retrypilot", features.RetryPilotMiddleware(rp))
	}

	// Spend tracking
	caps := buildCaps(cfg)
	add("spend", features.SpendMiddleware(features.SpendConfig{
		Counter: counter,
		Caps:    caps,
	}))

	// Cost warnings
	add("costwarn", features.CostWarnMiddleware(db.Conn()))

	// Cap enforcement
	add("caps", features.CapsMiddleware(caps, counter))

	// Output cap
	if cfg.OutputCap.Enabled {
		oc := features.NewOutputCap(cfg.OutputCap)
		add("outputcap", features.OutputCapMiddleware(oc))
	}

	// Idle kill
	if cfg.IdleKill.Enabled {
		ik := features.NewIdleKill(cfg.IdleKill)
		add("idlekill", features.IdleKillMiddleware(ik))
	}

	// Request logging
	add("logging", features.LoggingMiddleware(features.LoggingConfig{
		StoreBodies: cfg.Logging.StoreBodies,
		MaxBodySize: 8192,
		DB:          db,
	}))

	// Response headers
	add("responseheaders", features.ResponseHeadersMiddleware())

	// Dev proxy
	if cfg.DevProxy.Enabled {
		dp := features.NewDevProxy(cfg.DevProxy)
		add("devproxy", features.DevProxyMiddleware(dp))
	}

	// Prompt slim
	if cfg.PromptSlim.Enabled {
		ps := features.NewPromptSlim(cfg.PromptSlim)
		add("promptslim", features.PromptSlimMiddleware(ps))
	}

	// Local sync
	if cfg.LocalSync.Enabled {
		ls := features.NewLocalSync(cfg.LocalSync)
		add("localsync", features.LocalSyncMiddleware(ls))
	}

	return mw, failoverRouter, modelAliasState
}

// registerProxyTraceRoutes adds basic read-only trace endpoints for OSS.
func registerProxyTraceRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("GET /api/traces", func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		rows, err := conn.Query(`SELECT id, timestamp, model, provider, status, latency_ms,
			input_tokens, output_tokens, cost_usd, cached
			FROM requests ORDER BY timestamp DESC LIMIT ?`, limit)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		var traces []map[string]any
		for rows.Next() {
			var id, ts, model, prov string
			var status, latency, inTok, outTok int
			var cost float64
			var cached int
			if err := rows.Scan(&id, &ts, &model, &prov, &status, &latency, &inTok, &outTok, &cost, &cached); err != nil {
				continue
			}
			traces = append(traces, map[string]any{
				"id": id, "timestamp": ts, "model": model, "provider": prov,
				"status": status, "latency_ms": latency,
				"input_tokens": inTok, "output_tokens": outTok,
				"cost_usd": cost, "cached": cached == 1,
			})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[traces] rows error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"traces": traces, "count": len(traces)})
	})

	mux.HandleFunc("GET /api/traces/spend", func(w http.ResponseWriter, r *http.Request) {
		var totalCost float64
		var totalReqs int
		row := conn.QueryRow(`SELECT COALESCE(SUM(cost_usd),0), COUNT(*) FROM requests WHERE timestamp > datetime('now', '-30 days')`)
		if err := row.Scan(&totalCost, &totalReqs); err != nil {
			log.Printf("[traces] spend query error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"period":         "30d",
			"total_cost_usd": totalCost,
			"total_requests": totalReqs,
		})
	})
}

// proxyAppShim provides proxy management routes for the OSS binary.
type proxyAppShim struct {
	conn       *sql.DB
	toggle     *toggle.Registry
	failover   *features.FailoverRouter
	modelAlias *features.ModelAliasState
}

func (a *proxyAppShim) Migrate(conn *sql.DB) error {
	_, err := conn.Exec(`CREATE TABLE IF NOT EXISTS proxy_modules (
		name TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL DEFAULT 1,
		config TEXT DEFAULT '{}',
		updated_at TEXT DEFAULT (datetime('now'))
	)`)
	return err
}

func (a *proxyAppShim) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/proxy/modules", func(w http.ResponseWriter, r *http.Request) {
		states := a.toggle.KnownModules()
		type mod struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		var mods []mod
		for name, enabled := range states {
			mods = append(mods, mod{Name: name, Enabled: enabled})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"modules": mods, "count": len(mods)})
	})

	mux.HandleFunc("PUT /api/proxy/modules/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
			return
		}
		a.toggle.Set(name, req.Enabled)
		enabled := 0
		if req.Enabled {
			enabled = 1
		}
		_, dbErr := a.conn.Exec(`INSERT INTO proxy_modules (name, enabled, updated_at) VALUES (?, ?, datetime('now'))
			ON CONFLICT(name) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
			name, enabled)
		if dbErr != nil {
			log.Printf("[proxy] module persist error: %v", dbErr)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"name": name, "enabled": req.Enabled})
	})

	if a.modelAlias != nil {
		mux.HandleFunc("GET /api/proxy/aliases", func(w http.ResponseWriter, r *http.Request) {
			aliases := a.modelAlias.List()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"aliases": aliases})
		})

		mux.HandleFunc("PUT /api/proxy/aliases", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Alias string `json:"alias"`
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
				return
			}
			a.modelAlias.Set(req.Alias, req.Model)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"alias": req.Alias, "model": req.Model})
		})
	}
}
