<div align="center">

# ═══ STOCKYARD ═══

### 150 self-hosted developer tools. Single binary each. SQLite storage.

**Wrangle your Stack.**

[Website](https://stockyard.dev) · [All 150 Tools](https://stockyard.dev/tools/) · [Complete Bundle](https://stockyard.dev/complete/) · [Docs](https://stockyard.dev/docs/) · [Changelog](https://stockyard.dev/changelog/)

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/Proxy-Apache_2.0-4CAF50)](LICENSE-APACHE)
[![License](https://img.shields.io/badge/Platform-BSL_1.1-E8753A)](LICENSE)
[![Tools](https://img.shields.io/badge/Tools-150-D4A843)](https://stockyard.dev/tools/)
[![Modules](https://img.shields.io/badge/Modules-76-E8753A)](https://stockyard.dev/modules/)
[![Providers](https://img.shields.io/badge/Providers-16-C4A87A)](https://stockyard.dev/providers/)

</div>

---

150 focused self-hosted tools — webhook inboxes, status pages, feature flags, error tracking, analytics, link shorteners, secret managers, and more. Each tool is a single Go binary (~12MB) with embedded SQLite. No Docker, no Postgres, no Redis, no vendor lock-in. Your data lives on your server.

Plus an LLM proxy platform with 76 middleware modules and 16 providers.

**All 150 tools for $29/mo.** Free tier on every tool. [stockyard.dev/complete](https://stockyard.dev/complete/)

## Install Any Tool

```bash
# Install a single tool
curl -fsSL stockyard.dev/corral/install.sh | sh
stockyard-corral
# → Dashboard: http://localhost:8760/ui

# Install multiple tools
curl -fsSL stockyard.dev/install-tools.sh | sh -s -- headcount paddock saltlick

# Or use a preset bundle
curl -fsSL stockyard.dev/install-tools.sh | sh -s -- --starter    # 5 popular tools
curl -fsSL stockyard.dev/install-tools.sh | sh -s -- --devtools   # 8 developer tools
curl -fsSL stockyard.dev/install-tools.sh | sh -s -- --ops        # operations tools
```

### Interactive Menu

```bash
curl -fsSL stockyard.dev/install-menu.sh | sh
```

### Docker

```bash
docker run -d -p 4200:4200 -e OPENAI_API_KEY=sk-... ghcr.io/stockyard-dev/stockyard:latest
```

### Homebrew

```bash
brew tap stockyard-dev/tap
brew install stockyard-corral stockyard-headcount stockyard-paddock
```

## Popular Tools

| Tool | Replaces | What it does |
|------|----------|-------------|
| **[Corral](https://stockyard.dev/corral/)** | Hookdeck ($25/mo) | Webhook capture, inspect, replay, forward |
| **[Headcount](https://stockyard.dev/headcount/)** | Mixpanel ($20/mo) | Privacy-first user analytics, funnels, retention |
| **[Paddock](https://stockyard.dev/paddock/)** | Statuspage ($79/mo) | Public status page with incidents and notifications |
| **[Salt Lick](https://stockyard.dev/saltlick/)** | LaunchDarkly ($50/mo) | Feature flags, gradual rollouts, kill switches |
| **[Seismograph](https://stockyard.dev/seismograph/)** | Sentry ($26/mo) | Error tracking and exception aggregation |
| **[Lasso](https://stockyard.dev/lasso/)** | Bitly ($35/mo) | Link shortener with custom domains and QR codes |
| **[Strongbox](https://stockyard.dev/strongbox/)** | 1Password ($18/mo) | Encrypted secret manager with team access |
| **[Ledger](https://stockyard.dev/ledger/)** | QuickBooks ($15/mo) | Double-entry bookkeeping and reports |
| **[Sentinel](https://stockyard.dev/sentinel/)** | PagerDuty ($21/mo) | Alert manager with escalation policies |

[Browse all 150 tools →](https://stockyard.dev/tools/)

## Stockyard Complete — $29/mo

All 150 tools. One license key. Unlimited instances.

```
SaaS stack:  Sentry + Statuspage + LaunchDarkly + Bitly + Mixpanel = $210+/mo
Stockyard:   All 150 tools = $29/mo
```

- One license key unlocks Pro on every tool
- Run as many instances as you need
- Your rate locks in when you subscribe
- Cancel any time — tools keep running on free tier

[Get Complete →](https://stockyard.dev/complete/)

## LLM Platform

Stockyard also includes an LLM proxy platform — the tool that started it all. Point your `OPENAI_BASE_URL` at it and get cost tracking, caching, safety filters, rate limiting, audit trails, and automatic cost routing.

```bash
# Install the full platform
curl -fsSL https://stockyard.dev/install.sh | sh

export OPENAI_API_KEY=sk-...
stockyard start
# → Proxy:     http://localhost:4200/v1
# → Dashboard: http://localhost:4200/ui
```

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:4200/v1",
    api_key="your-openai-key"
)

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

- **76 middleware modules** — rate limiting, caching, failover, safety filters, cost caps, audit logging
- **16 LLM providers** — OpenAI, Anthropic, Gemini, Groq, Mistral, DeepSeek, Ollama, and more
- **~400ns proxy overhead** across the full middleware chain
- **~25MB binary**, embedded SQLite, zero external dependencies

[Platform details →](https://stockyard.dev/platform/) · [LLM Platform pricing →](https://stockyard.dev/pricing/)

## Architecture

```
┌─── Single Binary (~12MB per tool) ──────────────┐
│                                                   │
│  HTTP Server ──→ REST API ──→ SQLite (WAL mode)  │
│       │                                           │
│       └──→ Dashboard (embedded, serves at /ui)   │
│                                                   │
│  No external database. No Docker required.        │
│  Download, run, done.                             │
└───────────────────────────────────────────────────┘
```

## MCP Support

Connect MCP-compatible AI editors (Claude Desktop, Cursor, Windsurf) to Stockyard:

```json
{
  "mcpServers": {
    "stockyard": {
      "url": "http://localhost:4200/mcp/sse"
    }
  }
}
```

[MCP documentation →](https://stockyard.dev/mcp/)

## Shipped This Week

<!-- SHIPPED-START -->
**20 changes** in the last 7 days:

- **desktop: Stripe checkout + success page with .stockyard-license download** (2026-04-15)
- **site: serve /desktop/tools-index.json + .sig for desktop tool distribution** (2026-04-15)
- **site: serve /desktop/updates.json + .sig for desktop auto-update** (2026-04-15)
- **bus: lift from stockyard-wip, land at github.com/stockyard-dev/stockyard/bus** (2026-04-12)
- **admin: trial_drip suspects + mark-sent endpoints for Apr 12 cleanup** (2026-04-12)
- **trial_drip: atomic claim-before-send, fix duplicate-send bug** (2026-04-12)
- **trial_drip: add STOCKYARD_TRIAL_DRIP_DISABLED kill-switch** (2026-04-12)
- **trial_drip: also gate behind NURTURE_ENABLED** (2026-04-12)
- **nurture: disable by default, add 23h per-email rate limit** (2026-04-12)
- **internal/manifestmigrate: skeleton per TOOL-SCHEMA-DESIGN.md §5** (2026-04-12)
- **stockyard-manifest-validate: add --allow-single-file, unbreak tool-repo CI** (2026-04-12)
- **stockyard-manifest-validate: implement E040/E041/W042 cross-tool event graph** (2026-04-12)
- **stockyard-manifest-validate: raise test coverage 51% -> 62%** (2026-04-12)
- **stockyard-manifest-validate: add --github output mode** (2026-04-12)
- **step 2: add stockyard-manifest-validate (structural + type + catalog checks)** (2026-04-12)
- **quick_match: add 'accounting' + 'accounting firm' -> accountant-tax** (2026-04-11)
- **pricing: fix $7.99-with-150+-tools meta conflation, hide annual prices** (2026-04-11)
- **homepage: remove download button, add 'now let's make it yours' transition + chip row** (2026-04-11)
- **fix(recommender): route 'yoga studio' to yoga-studio bundle, not gym** (2026-04-10)
- **bundles: fix recommender keyword poaching, esp. the yogau2192gym hijack** (2026-04-10)

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

- [Quickstart (5 min)](https://stockyard.dev/docs/quickstart/)
- [All 150 Tools](https://stockyard.dev/tools/)
- [Configuration](https://stockyard.dev/docs/config/)
- [API Reference (360+ endpoints)](https://stockyard.dev/docs/api/)
- [SDKs (Python, Go, TypeScript)](https://stockyard.dev/docs/sdks/)
- [Benchmarks](https://stockyard.dev/benchmarks/)
- [API Sandbox](https://stockyard.dev/sandbox/)
- [vs LiteLLM](https://stockyard.dev/vs/litellm/)

## License

**Stockyard Proxy** — the core LLM proxy with 24 middleware modules, provider routing, model aliasing, caching, and failover — is open source under the [Apache License 2.0](LICENSE-APACHE).

**Stockyard Platform + Tools** — the full binary including all 76 modules, the dashboard, all 150 standalone tools, and all platform products — is licensed under the [Business Source License 1.1](LICENSE).

See [docs/licensing/open-core-boundary.md](docs/licensing/open-core-boundary.md) for the full boundary.

---

<div align="center">

**[stockyard.dev](https://stockyard.dev)** · Wrangle your Stack.

</div>
