<div align="center">

# ═══ STOCKYARD ═══

### Self-hosted LLM proxy and control plane in one Go binary

[Website](https://stockyard.dev) · [Docs](https://stockyard.dev/docs) · [Playground](https://stockyard.dev/playground) · [Changelog](https://stockyard.dev/changelog)

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-E8753A)](LICENSE)
[![Modules](https://img.shields.io/badge/Modules-58-E8753A)](https://stockyard.dev/products)
[![Providers](https://img.shields.io/badge/Providers-16-C4A87A)](https://stockyard.dev/docs)

</div>

---

Stockyard sits between your app and your LLM providers. Point your `OPENAI_BASE_URL` at it and you get cost tracking, caching, safety filters, rate limiting, audit trails, and observability — without adding any dependencies to your stack.

Single Go binary. Embedded SQLite. No Redis, no Postgres, no Docker required.

## See It Work in 60 Seconds

```bash
# Install (~15MB binary)
curl -fsSL https://stockyard.dev/install.sh | sh

# Start (all services on one port)
stockyard serve
# → Stockyard running on :4200
# → Proxy:   http://localhost:4200/v1
# → Console: http://localhost:4200/ui

# Send a request through the proxy
curl http://localhost:4200/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

That request just flowed through 58 middleware modules — rate limiter, cost tracker, safety filter, cache check, audit logger — and back. No configuration. Check the results:

```bash
# See the trace (cost, latency, tokens, provider)
curl http://localhost:4200/api/observe/traces?limit=1

# See the audit event (hash-chained, tamper-evident)
curl http://localhost:4200/api/trust/ledger?limit=1

# See cost attribution
curl http://localhost:4200/api/observe/costs
```

Or open `http://localhost:4200/ui` in your browser for the full dashboard.

## What You Get

| Component | What it does |
|-----------|-------------|
| **Proxy** | OpenAI-compatible gateway with 58 middleware modules and 16 provider integrations |
| **Observe** | Automatic request tracing, per-model cost dashboards, anomaly detection, alerts |
| **Trust** | SHA-256 hash-chained audit ledger, policy enforcement, compliance evidence export |
| **Studio** | Versioned prompt templates, A/B experiments, model benchmarks |
| **Forge** | DAG workflow engine for chaining LLM calls, transforms, and tool calls |
| **Exchange** | Config pack marketplace — install pre-built provider/module/workflow bundles |

All six run from the same binary on the same port. No microservices.

## When Stockyard Is NOT the Right Fit

- **You only need a thin API shim.** If you just want to swap between OpenAI and Anthropic with no middleware, [LiteLLM](https://github.com/BerriAI/litellm) is simpler.
- **You need 100+ provider integrations.** Stockyard supports 16 providers today. LiteLLM supports 100+.
- **You want managed-only.** Stockyard is self-hosted first. Managed cloud is available but the core experience is running the binary yourself.
- **You need a prompt-only tool.** If you only want prompt versioning and don't need a proxy, dedicated tools like PromptLayer or Humanloop are more focused.
- **You're not using LLMs in production yet.** Stockyard solves production problems — cost overruns, audit requirements, safety filtering, provider failover. If you're still prototyping, you don't need this yet.

## Architecture

```
Your App (OpenAI SDK)
        │
        ▼
┌─── STOCKYARD (:4200) ───────────────────────┐
│                                               │
│  Request → [58 middleware modules] → Provider │
│            rate limit → cache → safety →      │
│            cost cap → route → failover        │
│                                               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│  │ Observe  │ │  Trust   │ │  Studio  │     │
│  │ traces   │ │ audit    │ │ prompts  │     │
│  │ costs    │ │ policies │ │ A/B test │     │
│  └──────────┘ └──────────┘ └──────────┘     │
│  ┌──────────┐ ┌──────────┐                  │
│  │  Forge   │ │ Exchange │  SQLite (WAL)    │
│  │ workflows│ │ packs    │  ~15MB binary    │
│  └──────────┘ └──────────┘                  │
└───────────────────────────────────────────────┘
```

- **Single binary**, single port, single process. No orchestration.
- **Embedded SQLite** with WAL mode. No external database.
- **58 middleware modules**, each toggleable at runtime via `PUT /api/proxy/modules/{name}`.
- **16 LLM providers**: OpenAI, Anthropic, Gemini, Groq, Mistral, DeepSeek, Ollama, VLLM, AWS Bedrock, Azure OpenAI, Cohere, Together AI, Fireworks, Replicate, Perplexity, Hugging Face.
- **AES-256-GCM encryption** for all provider keys at rest.
- **400ns chain traversal overhead** across the full 58-module middleware chain ([benchmarks](https://stockyard.dev/benchmarks)). Total per-request overhead including module logic is <5ms.

## Use It With Any OpenAI SDK

```python
from openai import OpenAI

# Just change the base URL. Everything else stays the same.
client = OpenAI(
    base_url="http://localhost:4200/v1",
    api_key="your-openai-key"
)

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

Works with any OpenAI-compatible client — Python, Node, Go, curl. Switch providers by changing the model name.

## Toggle Modules at Runtime

```bash
# Check what's running
curl localhost:4200/api/proxy/modules | jq '.count'
# → 58

# Disable caching
curl -X PUT localhost:4200/api/proxy/modules/cache -d '{"enabled": false}'

# Enable rate limiting at 100 RPM
curl -X PUT localhost:4200/api/proxy/modules/ratelimit -d '{"enabled": true, "rpm": 100}'
```

## vs LiteLLM

LiteLLM is a Python LLM router. Stockyard is a router plus local observability and audit in one deploy.

| | Stockyard | LiteLLM |
|---|---|---|
| Language | Go | Python |
| Dependencies | Zero (single binary) | Redis + Postgres + Docker |
| Database | Embedded SQLite | External Postgres |
| Observability | Built-in (Observe) | External (Langfuse, etc.) |
| Audit trail | Hash-chained ledger | Not included |
| Prompt management | Built-in (Studio) | Not included |
| Workflow engine | Built-in (Forge) | Not included |
| Providers | 16 | 100+ |
| Self-hosted | `curl install`, 30s | Docker Compose |
| Binary size | ~15MB | ~200MB Docker image |

## Security

- **Provider keys encrypted at rest** — AES-256-GCM with random nonce per write
- **Hash-chained audit ledger** — every event cryptographically linked to the previous
- **API keys hashed** — SHA-256, never stored in plaintext
- **No key leakage** — provider keys never appear in logs, traces, or API responses

## Build from Source

```bash
git clone https://github.com/stockyard-dev/stockyard.git
cd stockyard
CGO_ENABLED=0 go build -o stockyard ./cmd/stockyard/
./stockyard serve
```

Requires Go 1.22+. No other dependencies.

## Pricing

Self-hosted free tier includes 20 modules, 3 providers, and 1,000 requests/month. Individual ($9.99/mo) unlocks all 58 modules, all 16 providers, and 10,000 requests/month. Pro and above are unlimited.

See [stockyard.dev/pricing](https://stockyard.dev/pricing) for details.

## Documentation

- [Getting Started (5 min)](https://stockyard.dev/guide)
- [API Reference](https://stockyard.dev/docs/api)
- [Module Catalog](https://stockyard.dev/products)
- [vs LiteLLM](https://stockyard.dev/vs/litellm)

## License

Stockyard is licensed under the [MIT License](LICENSE). Free to use, modify, and distribute.

---

<div align="center">

**[stockyard.dev](https://stockyard.dev)** — Where LLM traffic gets sorted.

</div>
