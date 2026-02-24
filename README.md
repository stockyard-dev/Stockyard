# Stockyard

**Where LLM traffic gets sorted.**

Stockyard is a Go proxy engine that sits between your app and any LLM provider. 20 tools for cost tracking, caching, rate limiting, failover, observability, and more. Each is a single binary with an embedded dashboard. No Python, no Redis, no Postgres, no dependencies.

```
Your App  -->  Stockyard  -->  OpenAI / Anthropic / Gemini / Groq / Ollama
                   |
             Dashboard (localhost/ui)
```

## Install (10 seconds)

```bash
brew install stockyard-dev/tap/stockyard
# or
npx @stockyard/stockyard
# or
curl -fsSL https://get.stockyard.dev | sh
# or
docker run -p 4000:4000 ghcr.io/stockyard-dev/stockyard
```

## Quick Start

```bash
export OPENAI_API_KEY=sk-...
stockyard
# Dashboard at http://localhost:4000/ui
```

Point any OpenAI SDK at `http://localhost:4000/v1` and everything works. Cost tracking, caching, rate limiting, failover — all automatic.

## Products

Every product is a standalone binary or part of the unified suite. All share the same OpenAI-compatible API surface.

### Cost and Billing

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| CostCap | `costcap` | 4100 | Spend tracking with hard/soft caps per model, key, time period |
| UsagePulse | `usagepulse` | 4740 | Per-user and per-team token metering with billing export |

### Caching

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| CacheLayer | `llmcache` | 4200 | Exact and semantic response caching with TTL |

### Reliability

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| FallbackRouter | `routefall` | 4400 | Provider failover with circuit breaker and health checks |
| RateShield | `rateshield` | 4500 | Rate limiting with token bucket and per-key limits |
| RetryPilot | `retrypilot` | 5500 | Intelligent retry with jitter, circuit breaker, model downgrade |
| KeyPool | `keypool` | 4700 | API key pooling and rotation |

### Quality and Safety

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| StructuredShield | `jsonguard` | 4300 | JSON schema validation with auto-retry on parse failure |
| EvalGate | `evalgate` | 4730 | Response quality scoring with auto-retry |
| PromptGuard | `promptguard` | 4710 | PII redaction and prompt injection detection |

### Prompt Engineering

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| PromptPad | `promptpad` | 4800 | Versioned prompt templates with A/B testing |
| TokenTrim | `tokentrim` | 4900 | Context window optimizer with truncation strategies |
| ContextPack | `contextpack` | 5400 | File, SQLite, and URL context injection |

### Routing

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| ModelSwitch | `modelswitch` | 4720 | Smart model routing by token count, patterns, and headers |

### Observability

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| LLMTap | `llmtap` | 5300 | Full analytics portal with p50, p95, p99 latency and cost trends |
| PromptReplay | `promptreplay` | 4600 | Request logging, replay, and export |
| StreamSnap | `streamsnap` | 5200 | SSE stream capture, replay, and TTFT metrics |

### Async and Multi-Model

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| BatchQueue | `batchqueue` | 5000 | Async request queue with concurrency control |
| MultiCall | `multicall` | 5100 | Multi-model consensus and comparison |

### Suite

| Product | Binary | Port | What it does |
|---------|--------|------|-------------|
| **Stockyard Suite** | `stockyard` | 4000 | All 20 products in one binary |

## Configuration

```yaml
# stockyard.yaml
listen: ":4000"

providers:
  - name: openai
    url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
  - name: anthropic
    url: https://api.anthropic.com/v1
    api_key: ${ANTHROPIC_API_KEY}
    adapter: anthropic

cache:
  enabled: true
  ttl: 1h
  semantic: true

cost:
  daily_limit: 50.00
  alert_threshold: 0.8

rate_limit:
  requests_per_minute: 60
  per_key: true

fallback:
  strategy: priority
  providers: [openai, anthropic]
```

## Providers

Works with 17+ LLM providers out of the box:

OpenAI, Anthropic, Google Gemini, Groq, Ollama, Mistral, Cohere, DeepSeek, xAI/Grok, Together AI, Fireworks AI, Perplexity, Replicate, Amazon Bedrock, Azure OpenAI, Google Vertex AI, OpenRouter

## Architecture

- **Language:** Go, no CGO, single static binary
- **Storage:** SQLite via modernc.org/sqlite, no external database
- **Dashboard:** Preact and TypeScript embedded via go:embed
- **API:** OpenAI-compatible (/v1/chat/completions, /v1/completions, /v1/embeddings)
- **Streaming:** Full SSE pass-through with mid-stream token counting
- **Config:** YAML with environment variable interpolation

## Pricing

| Tier | Individual Product | Suite (20 products) |
|------|-------------------|-------------------|
| Free | Limited usage | 5 products max |
| Starter | $9/mo | $19/mo |
| Pro | $29/mo | $59/mo |
| Team | $79/mo | $149/mo |

Suite at $59/mo = $2.95 per tool. Buying 3 individual products costs more than the entire suite.

## Documentation

Full docs at [stockyard.dev/docs](https://stockyard.dev/docs):

- [Installation](https://stockyard.dev/docs/install)
- [Quick Start](https://stockyard.dev/docs/quickstart)
- [Configuration](https://stockyard.dev/docs/config)
- [API Reference](https://stockyard.dev/docs/api)
- [Deployment](https://stockyard.dev/docs/deploy)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).
