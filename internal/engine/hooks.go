package engine

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// appHooksMiddleware wraps every proxy request and writes:
//   - A trace row into observe_traces + cost rollup into observe_cost_daily
//   - An audit event into trust_ledger (append-only hash chain)
//
// This is the outermost middleware — it sees every request and response.
func appHooksMiddleware(conn *sql.DB) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()
			traceID := genTraceID()

			// Call the rest of the chain
			resp, err := next(ctx, req)

			duration := time.Since(start)
			go recordObserveTrace(conn, traceID, req, resp, err, duration)
			go recordTrustEvent(conn, traceID, req, resp, err, duration)

			return resp, err
		}
	}
}

// recordObserveTrace writes a trace + daily cost rollup to Observe tables.
func recordObserveTrace(conn *sql.DB, traceID string, req *provider.Request, resp *provider.Response, reqErr error, dur time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[observe-hook] panic: %v", r)
		}
	}()

	prov := req.Provider
	model := req.Model
	status := "ok"
	var tokIn, tokOut int64
	var costUSD float64

	if reqErr != nil {
		status = "error"
	}
	if resp != nil {
		if resp.Provider != "" {
			prov = resp.Provider
		}
		if resp.Model != "" {
			model = resp.Model
		}
		tokIn = int64(resp.Usage.PromptTokens)
		tokOut = int64(resp.Usage.CompletionTokens)
		if resp.CacheHit {
			status = "cache_hit"
		}
		// Rough cost estimate: $0.002 per 1K input tokens, $0.006 per 1K output tokens
		costUSD = float64(tokIn)/1000*0.002 + float64(tokOut)/1000*0.006
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := conn.Exec(`INSERT INTO observe_traces 
		(id, request_id, service, operation, provider, model, status, duration_ms, tokens_in, tokens_out, cost_usd, metadata_json, created_at) 
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		traceID, traceID, "proxy", "chat.completion", prov, model, status,
		dur.Milliseconds(), tokIn, tokOut, costUSD, "{}", now)
	if err != nil {
		// Table might not exist if apps aren't registered — silent skip
		return
	}

	// Cost rollup
	today := time.Now().UTC().Format("2006-01-02")
	conn.Exec(`INSERT INTO observe_cost_daily (date, provider, model, requests, tokens_in, tokens_out, cost_usd) 
		VALUES (?,?,?,1,?,?,?) 
		ON CONFLICT(date, provider, model) DO UPDATE SET 
			requests=requests+1, tokens_in=tokens_in+excluded.tokens_in, 
			tokens_out=tokens_out+excluded.tokens_out, cost_usd=cost_usd+excluded.cost_usd`,
		today, prov, model, tokIn, tokOut, costUSD)
}

// recordTrustEvent appends to the immutable audit ledger.
func recordTrustEvent(conn *sql.DB, traceID string, req *provider.Request, resp *provider.Response, reqErr error, dur time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[trust-hook] panic: %v", r)
		}
	}()

	action := "proxy.request"
	resource := req.Model
	actor := req.UserID
	if actor == "" {
		actor = req.ClientIP
	}

	status := "ok"
	if reqErr != nil {
		status = "error"
		action = "proxy.error"
	}

	detail := fmt.Sprintf(`{"trace_id":"%s","provider":"%s","model":"%s","status":"%s","duration_ms":%d}`,
		traceID, req.Provider, req.Model, status, dur.Milliseconds())

	// Get previous hash for chain
	var prevHash string
	conn.QueryRow("SELECT hash FROM trust_ledger ORDER BY id DESC LIMIT 1").Scan(&prevHash)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prevHash, "proxy.request", action, resource, detail, now)
	h := sha256.Sum256([]byte(hashInput))
	hash := hex.EncodeToString(h[:])

	_, err := conn.Exec(`INSERT INTO trust_ledger 
		(event_type, actor, resource, action, detail_json, prev_hash, hash, created_at) 
		VALUES (?,?,?,?,?,?,?,?)`,
		"proxy.request", actor, resource, action, detail, prevHash, hash, now)
	if err != nil {
		// Table might not exist — silent skip
		return
	}
}

// seedProxyModules populates the proxy_modules table from active feature flags
// so the /api/proxy/modules endpoint returns real data.
func seedProxyModules(conn *sql.DB, pc ProductConfig) {
	var count int
	conn.QueryRow("SELECT COUNT(*) FROM proxy_modules").Scan(&count)
	if count > 0 {
		return // Already seeded
	}

	type mod struct {
		name     string
		category string
		enabled  bool
		priority int
	}

	modules := []mod{
		// Routing
		{"fallbackrouter", "routing", pc.Features.Failover, 10},
		{"modelswitch", "routing", pc.Features.ModelSwitch, 11},
		{"regionroute", "routing", pc.Features.RegionRoute, 12},
		{"localsync", "routing", pc.Features.LocalSync, 13},
		{"abrouter", "routing", pc.Features.ABRouter, 14},
		// Caching
		{"cachelayer", "caching", pc.Features.Cache, 20},
		{"embedcache", "caching", pc.Features.EmbedCache, 21},
		{"semanticcache", "caching", pc.Features.SemanticCache, 22},
		// Cost
		{"costcap", "cost", pc.Features.SpendCaps, 30},
		{"tierdrop", "cost", pc.Features.TierDrop, 31},
		{"idlekill", "cost", pc.Features.IdleKill, 32},
		{"outputcap", "cost", pc.Features.OutputCap, 33},
		{"usagepulse", "cost", pc.Features.UsagePulse, 34},
		// Rate
		{"rateshield", "rate", pc.Features.RateLimiting, 40},
		// Keys
		{"keypool", "keys", pc.Features.KeyPool, 50},
		// Transform
		{"promptslim", "transform", pc.Features.PromptSlim, 60},
		{"tokentrim", "transform", pc.Features.TokenTrim, 61},
		{"contextpack", "transform", pc.Features.ContextPack, 62},
		{"chatmem", "transform", pc.Features.ChatMem, 63},
		{"langbridge", "transform", pc.Features.LangBridge, 64},
		{"voicebridge", "transform", pc.Features.VoiceBridge, 65},
		// Validate
		{"structuredshield", "validate", pc.Features.Validation, 70},
		{"evalgate", "validate", pc.Features.EvalGate, 71},
		{"codefence", "validate", pc.Features.CodeFence, 72},
		// Safety
		{"promptguard", "safety", pc.Features.PromptGuard, 80},
		{"toxicfilter", "safety", pc.Features.ToxicFilter, 81},
		{"guardrail", "safety", pc.Features.GuardRail, 82},
		{"agegate", "safety", pc.Features.AgeGate, 83},
		{"hallucicheck", "safety", pc.Features.HalluciCheck, 84},
		{"secretscan", "safety", pc.Features.SecretScan, 85},
		{"agentguard", "safety", pc.Features.AgentGuard, 86},
		// Shims
		{"anthrofit", "shims", pc.Features.AnthroFit, 90},
		{"geminishim", "shims", pc.Features.GeminiShim, 91},
		// Stream
		{"streamsnap", "stream", pc.Features.StreamSnap, 100},
		// Multimodal
		{"imageproxy", "multimodal", pc.Features.ImageProxy, 110},
		// Tenant
		{"tenantwall", "tenant", pc.Features.TenantWall, 120},
		{"ipfence", "tenant", pc.Features.IPFence, 121},
		// Observability
		{"llmtap", "observe", pc.Features.LLMTap, 130},
		{"tracelink", "observe", pc.Features.TraceLink, 131},
		{"alertpulse", "observe", pc.Features.AlertPulse, 132},
		{"driftwatch", "observe", pc.Features.DriftWatch, 133},
		// Trust
		{"compliancelog", "trust", pc.Features.ComplianceLog, 140},
		{"feedbackloop", "trust", pc.Features.FeedbackLoop, 141},
		// Studio
		{"promptpad", "studio", pc.Features.PromptPad, 150},
		{"promptlint", "studio", pc.Features.PromptLint, 151},
		{"approvalgate", "studio", pc.Features.ApprovalGate, 152},
		// Forge
		{"batchqueue", "forge", pc.Features.BatchQueue, 160},
		{"multicall", "forge", pc.Features.MultiCall, 161},
		{"mockllm", "forge", pc.Features.MockLLM, 162},
		// Exchange
		{"devproxy", "exchange", pc.Features.DevProxy, 170},
	}

	for _, m := range modules {
		enabled := 0
		if m.enabled {
			enabled = 1
		}
		conn.Exec("INSERT OR IGNORE INTO proxy_modules (name, category, enabled, priority) VALUES (?,?,?,?)",
			m.name, m.category, enabled, m.priority)
	}

	log.Printf("[proxy] seeded %d modules into proxy_modules table", len(modules))
}

// seedProxyProviders populates the proxy_providers table from configured providers.
func seedProxyProviders(conn *sql.DB, providers map[string]provider.Provider) {
	var count int
	conn.QueryRow("SELECT COUNT(*) FROM proxy_providers").Scan(&count)
	if count > 0 {
		return
	}

	for name := range providers {
		conn.Exec("INSERT OR IGNORE INTO proxy_providers (name, status) VALUES (?, 'active')", name)
	}

	if len(providers) > 0 {
		log.Printf("[proxy] seeded %d providers into proxy_providers table", len(providers))
	}
}

func genTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "tr_" + hex.EncodeToString(b)
}
