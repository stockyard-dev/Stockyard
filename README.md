<div align="center">

# ═══ STOCKYARD ═══

### Self-hosted LLM proxy, tracing, and security in one Go binary

[Website](https://stockyard.dev) · [Docs](https://stockyard.dev/docs) · [Quickstart](https://stockyard.dev/docs/quickstart) · [Changelog](https://stockyard.dev/changelog)

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/Proxy-Apache_2.0-4CAF50)](LICENSE-APACHE)
[![License](https://img.shields.io/badge/Platform-BSL_1.1-E8753A)](LICENSE)
[![Modules](https://img.shields.io/badge/Modules-76-E8753A)](https://stockyard.dev/products)
[![Providers](https://img.shields.io/badge/Providers-40-C4A87A)](https://stockyard.dev/docs)

</div>

---

Stockyard sits between your app and your LLM providers. Point your `OPENAI_BASE_URL` at it and you get cost tracking, caching, safety filters, rate limiting, audit trails, and automatic cost routing — without adding any dependencies to your stack.

Single Go binary (~25MB). Embedded SQLite. No Redis, no Postgres, no Docker required.

## Install → Start → First Trace

<p align="center">
  <img src="internal/site/static/assets/marketing/install-demo.gif" alt="Terminal showing Stockyard install, startup, and first traced request with automatic cost routing" width="800">
</p>

```bash
# Install (~25MB binary)
curl -fsSL https://stockyard.dev/install.sh | sh

# Start (all services on one port)
export OPENAI_API_KEY=sk-...
stockyard start
# → Stockyard running on :7749
# → Proxy:     http://localhost:7749/v1
# → Dashboard: http://localhost:7749/ui
```

```bash
# Send a request through the proxy
curl http://localhost:7749/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

That request just flowed through 76 middleware modules — rate limiter, cost tracker, safety filter, cache check, audit logger — and back. Check the results:

```bash
# See the trace (cost, latency, tokens, provider)
curl http://localhost:7749/api/lookout/traces?limit=1

# See the audit event (hash-chained, tamper-evident)
curl http://localhost:7749/api/brand/ledger?limit=1

# See cost attribution
curl http://localhost:7749/api/lookout/costs
```

Or open `http://localhost:7749/ui` for the full dashboard.

## What's Free

### Open Source Proxy (Apache 2.0)

The proxy core is fully open source. Build it yourself or download the binary:

- **Proxy engine** — route to 40 LLM providers through one OpenAI-compatible API
- **24 middleware modules** — caching, failover, rate limiting, model aliasing, spend tracking, request logging
- **Provider adapters** — OpenAI, Anthropic, Gemini, Groq, DeepSeek, Mistral, Ollama, and more
- **Trace API** — per-request cost, latency, and token tracking via JSON endpoints
- **Module management** — toggle middleware at runtime via API

```bash
go build -o stockyard-proxy ./cmd/stockyard-proxy/
```

### Free Tier of Full Platform (BSL 1.1)

The commercial binary includes everything above plus:

- **Lookout** — tracing dashboards, per-model cost views, anomaly detection
- **Brand** — SHA-256 hash-chained audit ledger, policy enforcement, compliance evidence
- **Tack Room** — versioned prompt templates, A/B experiments
- **Admin dashboard** — visual console for all features
- **Team isolation** — per-team API keys with isolated logs and spend tracking
- **Drover** — automatic cost routing, 100 decisions/day free
- **Feral quickscan** — 5 adversarial probes, instant A-F security grade
- Unlimited requests. No credit card. Self-hosted on your infrastructure.

## What Unlocks with Paid Tiers

29 specialized tools unlock as your needs grow ($29.99–$499.99/mo):

| Tier | Price | Key Products |
|------|-------|-------------|
| **Individual** | $29.99/mo | Lasso (request replay), Auction (model bidding), Doubt, Verdikt, Relic |
| **Pro** | $99.99/mo | Drover unlimited, Breed (prompt evolution), Stampede, Fault, Morph |
| **Team** | $299.99/mo | Feral full suite (29 probes), Phantom, Crucible, RBAC, 5 seats |
| **Enterprise** | $499.99/mo | All 29 products, Cortex, Mycelium, Ramrod, SSO, SLA |

Paid tiers add capabilities, not capacity. See [stockyard.dev/pricing](https://stockyard.dev/pricing).

## When Stockyard Is NOT the Right Fit

- **You only need a thin API shim.** If you just want to swap between OpenAI and Anthropic with no middleware, [LiteLLM](https://github.com/BerriAI/litellm) is simpler.
- **You need 100+ provider integrations.** Stockyard supports 40 providers. LiteLLM supports 100+.
- **You want managed-only.** Stockyard is self-hosted. You run the binary on your infrastructure.
- **You need MIT licensing.** Stockyard is BSL 1.1 (source-available, free to use, can't build a competing proxy service).
- **You're not using LLMs in production yet.** Stockyard solves production problems — cost overruns, audit requirements, safety filtering, provider failover.

## Architecture

```
Your App (OpenAI SDK)
        │
        ▼
┌─── STOCKYARD (:7749) ──────────────────────────┐
│                                                  │
│  Request → [76 middleware modules] → Provider    │
│            rate limit → cache → safety →         │
│            cost cap → route → failover           │
│                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │ Lookout  │ │  Brand   │ │Tack Room │        │
│  │ traces   │ │ audit    │ │ prompts  │        │
│  │ costs    │ │ policies │ │ A/B test │        │
│  └──────────┘ └──────────┘ └──────────┘        │
│  ┌──────────┐ ┌──────────┐                     │
│  │  Drover  │ │  Feral   │  SQLite (WAL)       │
│  │ routing  │ │ red-team │  ~25MB binary        │
│  └──────────┘ └──────────┘  29 products         │
└──────────────────────────────────────────────────┘
```

- **Single binary**, single port, single process. No orchestration.
- **Embedded SQLite** with WAL mode via `modernc.org/sqlite` (pure Go, no CGO).
- **76 middleware modules**, each toggleable at runtime via the API.
- **40 LLM providers**: OpenAI, Anthropic, Gemini, Groq, Mistral, DeepSeek, Ollama, vLLM, Azure OpenAI, Cohere, Together AI, Fireworks, Replicate, Perplexity, Hugging Face, xAI, OpenRouter, DeepInfra, NVIDIA NIM, Cerebras, SambaNova, AI21, and 18 more.
- **AES-256-GCM encryption** for all provider keys at rest.
- **~400ns proxy overhead** across the full middleware chain ([benchmarks](https://stockyard.dev/benchmarks)).

## Use It With Any OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:7749/v1",
    api_key="your-openai-key"
)

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

Works with any OpenAI-compatible client — Python, Node, Go, curl.

## Team Isolation

Give each team or project its own API keys. Requests are automatically isolated in logs, traces, spend tracking, and the audit ledger.

```bash
# Create a team
curl -X POST http://localhost:7749/api/teams \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d '{"name": "Frontend"}'

# Generate a team key
curl -X POST http://localhost:7749/api/teams/1/keys \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d '{"name": "production"}'
# → {"key": "sk-sy-...", "team_id": 1, ...}

# Use the team key — requests are automatically scoped
curl http://localhost:7749/v1/chat/completions \
  -H "Authorization: Bearer sk-sy-TEAM_KEY" \
  -d '{"model": "gpt-4o", "messages": [...]}'

# View team-scoped spend
curl http://localhost:7749/api/teams/1/spend \
  -H "X-Admin-Key: $ADMIN_KEY"
```

One team cannot see another team's data. The dashboard includes a team picker that filters all views.

## vs LiteLLM

| | Stockyard | LiteLLM |
|---|---|---|
| Language | Go | Python |
| Dependencies | Zero (single binary) | Redis + Postgres |
| Observability | Built-in (Lookout) | External (Langfuse, etc.) |
| Audit trail | Hash-chained ledger (Brand) | Not included |
| Red-team testing | Built-in (Feral, 29 probes) | Not included |
| Auto cost routing | Built-in (Drover) | Basic fallback |
| Request replay | Built-in (Lasso) | Not included |
| Prompt evolution | Built-in (Breed) | Not included |
| Providers | 40 | 100+ |
| Binary size | ~25MB | ~200MB+ Docker image |
| License | BSL 1.1 | MIT |

## Security

- **Provider keys encrypted at rest** — AES-256-GCM with PBKDF2-derived key (100K iterations)
- **Hash-chained audit ledger** — every event cryptographically linked to the previous
- **No key leakage** — provider keys never appear in logs, traces, or API responses
- **Security headers** — HSTS, CSP, X-Frame-Options, X-Content-Type-Options

## Shipped This Week

<!-- SHIPPED-START -->
**20 changes** in the last 7 days:

- **Site restructure: Wrangle your Stack** (2026-04-01)
- **fix: address message sprawl and boundary confusion (GPT review)** (2026-04-01)
- **seo: full polish pass — schema, titles, descriptions, sitemap, breadcrumbs** (2026-03-31)
- **fix: add Dashboard tab to Gate, Fence, Brand product pages** (2026-03-31)
- **feat: SVG mockups for all 5 tool product pages** (2026-03-31)
- **feat: conversion pass — 9 SEO pages, product page improvements, post-purchase flow** (2026-03-31)
- **fix: billing/success page — add license key email instructions** (2026-03-31)
- **feat: Stripe checkout for 5 tool Pro plans** (2026-03-31)
- **feat: licensegen CLI for issuing Ed25519-signed tool license keys** (2026-03-31)
- **feat: Free/Pro tier copy across all 5 product pages + pricing page** (2026-03-31)
- **fix: serve install.sh files via dedicated handlers (not pages loop)** (2026-03-31)
- **feat: install scripts, v0.1.0 releases wired, sitemap +10 URLs** (2026-03-31)
- **feat: product family site — 5 landing pages, 5 docs, nav dropdown, homepage section, pricing section** (2026-03-31)
- **Stockyard Tools: 5 new product landing pages + index** (2026-03-31)
- **feat: MCP server — 5 new management tools (was only chat)** (2026-03-31)
- **feat: studio promote winner + version rollback** (2026-03-31)
- **feat: forge scheduled workflows — cron-style background execution** (2026-03-31)
- **feat: observe CSV cost report export — 4 sections for finance teams** (2026-03-31)
- **feat: trust evidence export + policy templates, exchange pack ratings** (2026-03-31)
- **feat: forge workflow templates — 5 built-in patterns + clone API** (2026-03-31)

_See [full changelog](https://stockyard.dev/changelog/) for details._
<!-- SHIPPED-END -->

## Build from Source

```bash
git clone https://github.com/stockyard-dev/Stockyard.git
cd Stockyard

# Open-source proxy (Apache 2.0)
CGO_ENABLED=0 go build -o stockyard-proxy ./cmd/stockyard-proxy/
./stockyard-proxy

# Full platform (BSL 1.1)
CGO_ENABLED=0 go build -o stockyard ./cmd/stockyard/
./stockyard
```

Requires Go 1.22+. No other dependencies.

## Documentation

- [Quickstart (5 min)](https://stockyard.dev/docs/quickstart)
- [Configuration](https://stockyard.dev/docs/config)
- [SDKs (Python, Go, TypeScript)](https://stockyard.dev/docs/sdks)
- [API Reference (360+ endpoints)](https://stockyard.dev/docs/api)
- [Team Key Isolation](https://stockyard.dev/docs/team)
- [Products](https://stockyard.dev/products)
- [Benchmarks](https://stockyard.dev/benchmarks)
- [vs LiteLLM](https://stockyard.dev/vs/litellm)

## License

Stockyard uses a dual-license model:

**Stockyard Proxy** — the core LLM proxy with provider routing, model aliasing, caching, failover, rate limiting, spend tracking, and request logging — is open source under the [Apache License 2.0](LICENSE-APACHE).

**Stockyard Platform** — the full binary including Observe (tracing dashboards), Trust (audit ledger), Studio (prompt management), Forge (workflows), Exchange (config packs), the admin dashboard, and all 29 platform products — is licensed under the [Business Source License 1.1](LICENSE).

### Building

```bash
# Open-source proxy only
go build -o stockyard-proxy ./cmd/stockyard-proxy/

# Full commercial platform
go build -o stockyard ./cmd/stockyard/
```

The OSS proxy is a complete, standalone product. The commercial platform adds advanced observability, compliance, orchestration, and team features.

See [docs/licensing/open-core-boundary.md](docs/licensing/open-core-boundary.md) for the full classification of what's in each binary.

---

<div align="center">

**[stockyard.dev](https://stockyard.dev)**

</div>
