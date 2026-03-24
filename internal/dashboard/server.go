// Package dashboard serves the embedded Preact SPA and SSE events.
//
//go:generate bash build.sh
package dashboard

import (
	"crypto/subtle"
	"embed"
	"net/http"
	"os"
	"strings"
)

//go:embed static
var staticFiles embed.FS

// Product display names
var productNames = map[string]string{
	"costcap":         "CostCap",
	"llmcache":        "CacheLayer",
	"jsonguard":       "StructuredShield",
	"routefall":       "FallbackRouter",
	"rateshield":      "RateShield",
	"promptreplay":    "PromptReplay",
	"keypool":         "KeyPool",
	"promptguard":     "PromptGuard",
	"modelswitch":     "ModelSwitch",
	"evalgate":        "EvalGate",
	"usagepulse":      "UsagePulse",
	"promptpad":       "PromptPad",
	"tokentrim":       "TokenTrim",
	"batchqueue":      "BatchQueue",
	"multicall":       "MultiCall",
	"streamsnap":      "StreamSnap",
	"llmtap":          "LLMTap",
	"contextpack":     "ContextPack",
	"retrypilot":      "RetryPilot",
	"toxicfilter":     "ToxicFilter",
	"compliancelog":   "ComplianceLog",
	"secretscan":      "SecretScan",
	"tracelink":       "TraceLink",
	"ipfence":         "IPFence",
	"embedcache":      "EmbedCache",
	"anthrofit":       "AnthroFit",
	"alertpulse":      "AlertPulse",
	"chatmem":         "ChatMem",
	"mockllm":         "MockLLM",
	"tenantwall":      "TenantWall",
	"idlekill":        "IdleKill",
	"agentguard":      "AgentGuard",
	"codefence":       "CodeFence",
	"hallucicheck":    "HalluciCheck",
	"tierdrop":        "TierDrop",
	"driftwatch":      "DriftWatch",
	"feedbackloop":    "FeedbackLoop",
	"abrouter":        "ABRouter",
	"guardrail":       "GuardRail",
	"geminishim":      "GeminiShim",
	"localsync":       "LocalSync",
	"devproxy":        "DevProxy",
	"promptslim":      "PromptSlim",
	"promptlint":      "PromptLint",
	"approvalgate":    "ApprovalGate",
	"outputcap":       "OutputCap",
	"agegate":         "AgeGate",
	"voicebridge":     "VoiceBridge",
	"imageproxy":      "ImageProxy",
	"langbridge":      "LangBridge",
	"contextwindow":   "ContextWindow",
	"regionroute":     "RegionRoute",
	"chainforge":      "ChainForge",
	"cronllm":         "CronLLM",
	"webhookrelay":    "WebhookRelay",
	"billsync":        "BillSync",
	"whitelabel":      "WhiteLabel",
	"trainexport":     "TrainExport",
	"synthgen":        "SynthGen",
	"diffprompt":      "DiffPrompt",
	"llmbench":        "LLMBench",
	"maskmode":        "MaskMode",
	"tokenmarket":     "TokenMarket",
	"llmsync":         "LLMSync",
	"clustermode":     "ClusterMode",
	"encryptvault":    "EncryptVault",
	"mirrortest":      "MirrorTest",
	"extractml":       "ExtractML",
	"tableforge":      "TableForge",
	"toolrouter":      "ToolRouter",
	"toolshield":      "ToolShield",
	"toolmock":        "ToolMock",
	"authgate":        "AuthGate",
	"scopeguard":      "ScopeGuard",
	"visionproxy":     "VisionProxy",
	"audioproxy":      "AudioProxy",
	"docparse":        "DocParse",
	"framegrab":       "FrameGrab",
	"sessionstore":    "SessionStore",
	"convofork":       "ConvoFork",
	"slotfill":        "SlotFill",
	"semanticcache":   "SemanticCache",
	"partialcache":    "PartialCache",
	"streamcache":     "StreamCache",
	"promptchain":     "PromptChain",
	"promptfuzz":      "PromptFuzz",
	"promptmarket":    "PromptMarket",
	"costpredict":     "CostPredict",
	"costmap":         "CostMap",
	"spotprice":       "SpotPrice",
	"loadforge":       "LoadForge",
	"snapshottest":    "SnapshotTest",
	"chaosllm":        "ChaosLLM",
	"datamap":         "DataMap",
	"consentgate":     "ConsentGate",
	"retentionwipe":   "RetentionWipe",
	"policyengine":    "PolicyEngine",
	"streamsplit":     "StreamSplit",
	"streamthrottle":  "StreamThrottle",
	"streamtransform": "StreamTransform",
	"modelalias":      "ModelAlias",
	"paramnorm":       "ParamNorm",
	"quotasync":       "QuotaSync",
	"errornorm":       "ErrorNorm",
	"cohorttrack":     "CohortTrack",
	"promptrank":      "PromptRank",
	"anomalyradar":    "AnomalyRadar",
	"envsync":         "EnvSync",
	"proxylog":        "ProxyLog",
	"clidash":         "CliDash",
	"embedrouter":     "EmbedRouter",
	"finetunetrack":   "FineTuneTrack",
	"agentreplay":     "AgentReplay",
	"summarizegate":   "SummarizeGate",
	"codelang":        "CodeLang",
	"personaswitch":   "PersonaSwitch",
	"warmpool":        "WarmPool",
	"edgecache":       "EdgeCache",
	"queuepriority":   "QueuePriority",
	"geoprice":        "GeoPrice",
	"tokenauction":    "TokenAuction",
	"canarydeploy":    "CanaryDeploy",
	"playbackstudio":  "PlaybackStudio",
	"webhookforge":    "WebhookForge",
	"stockyard":       "Stockyard",
}

// Register mounts the dashboard routes on the given ServeMux.
func Register(mux *http.ServeMux, product string) {
	adminKey := os.Getenv("STOCKYARD_ADMIN_KEY")

	// Read the template HTML at startup
	htmlBytes, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(fallbackHTML(product)))
		})
		return
	}

	// Inject product key and name into the template
	name := productNames[product]
	if name == "" {
		name = product
	}
	html := string(htmlBytes)
	html = strings.Replace(html, "__PRODUCT__", product, 1)
	html = strings.Replace(html, "__PRODUCT_NAME__", name, 1)
	rendered := []byte(html)

	// serveUI wraps the SPA handler with admin key auth (cookie or query param).
	// If STOCKYARD_ADMIN_KEY is not set, the dashboard is open (dev mode).
	serveUI := func(w http.ResponseWriter, r *http.Request) {
		if adminKey != "" {
			// Check cookie
			cookie, _ := r.Cookie("stockyard_session")
			if cookie == nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(adminKey)) != 1 {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(loginHTML))
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(rendered)
	}

	// Serve the SPA for /ui and /ui/ (all client-side routing)
	mux.HandleFunc("GET /ui", serveUI)
	mux.HandleFunc("GET /ui/", serveUI)

	// POST /ui/login — handle login form submission
	mux.HandleFunc("POST /ui/login", func(w http.ResponseWriter, r *http.Request) {
		if adminKey == "" {
			http.Redirect(w, r, "/ui", http.StatusFound)
			return
		}
		r.ParseForm()
		key := r.FormValue("key")
		if subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) == 1 {
			http.SetCookie(w, &http.Cookie{
				Name:     "stockyard_session",
				Value:    adminKey,
				Path:     "/ui",
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteStrictMode,
				MaxAge:   86400 * 7,
			})
			http.Redirect(w, r, "/ui", http.StatusFound)
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(loginHTML))
		}
	})

	// Serve the playground (public, no auth)
	pgBytes, pgErr := staticFiles.ReadFile("static/playground.html")
	if pgErr == nil {
		pgHTML := pgBytes
		mux.HandleFunc("GET /playground", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(pgHTML)
		})
		mux.HandleFunc("GET /playground/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(pgHTML)
		})
	}
}

const loginHTML = `<!DOCTYPE html>
<html><head><title>Stockyard — Login</title>
<style>body{font-family:system-ui;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#1a1a1a;color:#e0d5c8}
.login{background:#2a2a2a;padding:2rem;border-radius:8px;max-width:360px;width:100%}
h2{margin-top:0;color:#c87533}input{width:100%;padding:8px;margin:8px 0 16px;border:1px solid #555;border-radius:4px;background:#333;color:#e0d5c8;box-sizing:border-box}
button{width:100%;padding:10px;background:#c87533;color:#fff;border:none;border-radius:4px;cursor:pointer;font-size:1rem}
button:hover{background:#a85d22}</style></head>
<body><div class="login"><h2>Stockyard Dashboard</h2><p>Enter your admin key to continue.</p>
<form method="POST" action="/ui/login"><input type="password" name="key" placeholder="Admin Key" autofocus>
<button type="submit">Sign In</button></form></div></body></html>`

func fallbackHTML(product string) string {
	name := productNames[product]
	if name == "" {
		name = product
	}
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><title>` + name + `</title>
<style>*{margin:0;padding:0;box-sizing:border-box}body{font-family:system-ui;background:#0c0e14;color:#f0f1f4;display:flex;align-items:center;justify-content:center;min-height:100vh}
.c{text-align:center}h1{font-size:2rem;margin-bottom:.5rem}p{color:#8b90a0}.d{width:8px;height:8px;background:#34D399;border-radius:50%;display:inline-block;margin-right:8px}</style>
</head><body><div class="c"><h1><span class="d"></span>` + name + `</h1><p>Dashboard loading... Proxy is running.</p></div></body></html>`
}
