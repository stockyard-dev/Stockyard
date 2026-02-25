# Stockyard

**Where LLM traffic gets sorted.**

Six apps. One Go binary. Zero dependencies. The complete LLM infrastructure platform — proxy, observe, trust, studio, forge, and exchange.

```
Your App  →  Stockyard  →  OpenAI / Anthropic / Gemini / Groq / Ollama / 12 more
                 │
    Console (localhost:4200/ui)
    ├── Proxy     50+ middleware modules, runtime toggles
    ├── Observe   Traces, costs, alerts, anomaly detection
    ├── Trust     Hash-chained audit ledger, policies
    ├── Studio    Versioned prompts, A/B experiments
    ├── Forge     DAG workflow engine
    └── Exchange  Config pack marketplace
```

## Install

```bash
curl -sSL stockyard.dev/install | sh
```

Or build from source:

```bash
git clone https://github.com/stockyard-dev/stockyard
cd stockyard
go build -o stockyard ./cmd/stockyard
```

## Quickstart

```bash
# Start the platform (all 6 apps on port 4200)
stockyard

# Point your app at the proxy
export OPENAI_BASE_URL=http://localhost:4200/v1

# Make a request — goes through the full middleware chain
curl http://localhost:4200/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

Open `http://localhost:4200/ui` for the web console.

## The Platform

### Proxy (App 01)
50+ middleware modules in a chain, every one toggleable at runtime:

| Category | Modules |
|----------|---------|
| Routing | fallbackrouter, modelswitch, regionroute, abrouter, localsync |
| Caching | cachelayer, embedcache, semanticcache |
| Cost | costcap, tierdrop, rateshield, idlekill, outputcap, usagepulse |
| Safety | promptguard, toxicfilter, guardrail, agegate, hallucicheck, secretscan, agentguard |
| Transform | promptslim, tokentrim, contextpack, chatmem, langbridge, voicebridge |
| Validate | structuredshield, evalgate, codefence |
| Observe | llmtap, tracelink, alertpulse, driftwatch |
| Shims | anthrofit (Claude→OpenAI SDK), geminishim (Gemini→OpenAI SDK) |

Toggle any module at runtime:
```bash
# Disable a module (immediate, no restart)
curl -X PUT localhost:4200/api/proxy/modules/toxicfilter -d '{"enabled":false}'

# Re-enable it
curl -X PUT localhost:4200/api/proxy/modules/toxicfilter -d '{"enabled":true}'
```

### Observe (App 02)
Every proxy request is automatically traced with model, tokens, cost, latency, and status.

```bash
curl localhost:4200/api/observe/traces?limit=10  # Recent traces
curl localhost:4200/api/observe/costs             # Daily cost rollups
curl localhost:4200/api/observe/alerts            # Alert rules
curl localhost:4200/api/observe/anomalies         # Detected anomalies
```

### Trust (App 03)
Append-only audit ledger with SHA-256 hash chain. Every request gets a tamper-evident record.

```bash
curl localhost:4200/api/trust/ledger?limit=10     # Audit trail
curl localhost:4200/api/trust/policies             # Trust policies
```

### Studio (App 04)
Versioned prompt templates, A/B experiments, benchmarks, and snapshot comparison.

```bash
curl localhost:4200/api/studio/templates           # Prompt templates
curl localhost:4200/api/studio/experiments          # A/B experiments
```

### Forge (App 05)
DAG workflow engine. Chain LLM calls with dependency ordering and template variables.

```bash
# Create a multi-step workflow
curl -X POST localhost:4200/api/forge/workflows -d '{
  "slug": "draft-and-critique",
  "name": "Draft + Critique",
  "steps": [
    {"id":"draft","type":"llm","config":{"model":"gpt-4o-mini","prompt":"Write about {{input}}"}},
    {"id":"critique","type":"llm","depends_on":["draft"],
     "config":{"prompt":"Critique: {{steps.draft.output}}"}},
    {"id":"final","type":"transform","depends_on":["draft","critique"],
     "config":{"expression":"concat"}}
  ]
}'

# Run it
curl -X POST localhost:4200/api/forge/workflows/draft-and-critique/run \
  -d '{"input":"the future of AI"}'
```

### Exchange (App 06)
Config pack marketplace. Install providers, modules, routes, workflows, policies, and alerts in one click.

```bash
curl localhost:4200/api/exchange/packs                              # List packs
curl -X POST localhost:4200/api/exchange/packs/safety-essentials/install  # Install
curl -X DELETE localhost:4200/api/exchange/installed/1               # Uninstall
```

**6 starter packs included:** Safety Essentials, Cost Control, OpenAI Quickstart, Anthropic Quickstart, Multi-Provider Failover, Evaluation Suite.

## Auth

Set `STOCKYARD_ADMIN_KEY` to protect the management API:

```bash
export STOCKYARD_ADMIN_KEY=sk-your-secret-key
stockyard
```

With the key set:
- `/api/*` endpoints require `Authorization: Bearer <key>` or `X-Admin-Key: <key>`
- `/v1/*` proxy endpoints pass through (they use your LLM provider's key)
- `/health` and `/ui` remain open

Without the key, all endpoints are open (dev mode).

## Providers

Stockyard supports 17 LLM providers: OpenAI, Anthropic, Google Gemini, Groq, Mistral, Cohere, AI21, Together, Fireworks, Perplexity, Ollama, LM Studio, vLLM, Azure OpenAI, AWS Bedrock, Replicate, and DeepSeek.

The proxy is a transparent pass-through — set your provider's API key in the `Authorization` header and Stockyard forwards it upstream.

## API

69 REST endpoints across all 6 apps. Key endpoints:

```
GET  /health                              Health check
GET  /api/apps                            List apps
POST /v1/chat/completions                 Proxy LLM request
GET  /api/proxy/modules                   List modules
PUT  /api/proxy/modules/{name}            Toggle module
GET  /api/proxy/providers                 List providers
GET  /api/observe/traces                  Recent traces
GET  /api/observe/costs                   Cost rollups
POST /api/observe/alerts                  Create alert
GET  /api/trust/ledger                    Audit trail
GET  /api/studio/templates                Prompt templates
POST /api/forge/workflows                 Create workflow
POST /api/forge/workflows/{slug}/run      Run workflow
GET  /api/exchange/packs                  Available packs
POST /api/exchange/packs/{slug}/install   Install pack
```

## Why not LiteLLM?

| | Stockyard | LiteLLM |
|---|---|---|
| Language | Go | Python |
| Binary | Single static binary | pip install + runtime |
| Dependencies | Zero | Redis, Postgres |
| Platform | 6 integrated apps | One proxy |
| Middleware | 50+ toggleable modules | Limited callbacks |
| Memory | ~12MB | ~200MB+ |
| Cold start | <50ms | Seconds |

## Links

- **Website:** [stockyard.dev](https://stockyard.dev)
- **Cloud:** [stockyard.dev/cloud](https://stockyard.dev/cloud)
- **Docs:** [stockyard.dev/docs](https://stockyard.dev/docs)
- **Live demo:** [stockyard-production.up.railway.app](https://stockyard-production.up.railway.app/health)

## License

See [LICENSE](LICENSE).
