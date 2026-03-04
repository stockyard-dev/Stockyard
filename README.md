<div align="center">

# ═══ STOCKYARD ═══

### The complete LLM infrastructure platform

**Six apps. One binary. Zero dependencies.**

[Website](https://stockyard.dev) · [Documentation](https://stockyard.dev/docs) · [Changelog](https://stockyard.dev/changelog) · [Pricing](https://stockyard.dev/pricing)

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-BSL_1.1-E8753A)](LICENSE)
[![Deploy](https://img.shields.io/badge/Deploy_in-30s-C4A87A)](https://stockyard.dev/docs)
[![Modules](https://img.shields.io/badge/Modules-70-E8753A)](https://stockyard.dev/docs)
[![Providers](https://img.shields.io/badge/Providers-16-C4A87A)](https://stockyard.dev/docs)

</div>

---

## The Problem

Teams building with LLMs cobble together **5–10 separate tools** for basic proxy routing, observability, safety guardrails, and audit compliance. There are **134+ standalone middleware tools** in the ecosystem — each with its own runtime, database, config format, and failure modes.

The result: fragile stacks, opaque costs, zero audit trails, and weeks of integration work.

## The Solution

Stockyard replaces your entire LLM middleware stack with **one Go binary** and **embedded SQLite**. No Redis. No Postgres. No Docker compose files. Deploy in 30 seconds.

```bash
# Install
curl -fsSL https://stockyard.dev/install.sh | sh

# Run
stockyard serve

# That's it. All 6 apps are running.
```

## Six Apps, One Platform

| App | What it does | Key features |
|-----|-------------|--------------|
| **Proxy** | Gateway layer | 70 middleware modules, 16 providers, OpenAI-compatible endpoint |
| **Observe** | See everything | Automatic tracing, per-model cost attribution, anomaly detection |
| **Trust** | Immutable audit | SHA-256 hash-chained ledger, tamper-evident logging, policy engine |
| **Studio** | Prompt engineering | Version control, side-by-side diffing, A/B experimentation |
| **Forge** | Workflow engine | DAG-based orchestration, tool registry, visual builder |
| **Exchange** | Config marketplace | Pre-built packs, one-click install, share stacks across teams |

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Client SDK                     │
│            (OpenAI-compatible API)               │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│                 STOCKYARD PROXY                  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │  Rate   │→│  Cache  │→│  Route  │→ Provider  │
│  │ Limiter │ │         │ │         │  (16 LLMs) │
│  └─────────┘ └─────────┘ └─────────┘           │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │  Safety │ │  Cost   │ │ Guards  │            │
│  │ Filters │ │Controls │ │  Rails  │            │
│  └─────────┘ └─────────┘ └─────────┘           │
└──────────┬────────────────────┬─────────────────┘
           │                    │
    ┌──────▼──────┐     ┌──────▼──────┐
    │   OBSERVE   │     │    TRUST    │
    │   Traces    │     │  Hash-chain │
    │   Costs     │     │   Ledger    │
    │   Alerts    │     │  Policies   │
    └─────────────┘     └─────────────┘

    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │   STUDIO    │  │    FORGE    │  │  EXCHANGE   │
    │  Prompts    │  │  Workflows  │  │   Configs   │
    │  Versions   │  │    DAGs     │  │   Packs     │
    └─────────────┘  └─────────────┘  └─────────────┘

    ┌─────────────────────────────────────────────┐
    │           Embedded SQLite (WAL)              │
    └─────────────────────────────────────────────┘
```

## Quick Start

### OpenAI-Compatible Proxy

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="sk-stockyard-..."
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### 70 Middleware Modules — Runtime-Toggled

```bash
# Enable caching
curl -X PUT localhost:8080/api/proxy/modules/cache \
  -d '{"enabled": true, "ttl": 300}'

# Enable rate limiting
curl -X PUT localhost:8080/api/proxy/modules/rate-limit \
  -d '{"enabled": true, "rpm": 100}'
```

### Hash-Chain Audit

```bash
# Verify audit chain integrity
curl localhost:8080/api/trust/verify
# → {"valid": true, "events": 15847, "chain_intact": true}

# Export for compliance
curl localhost:8080/api/trust/export?format=csv > audit.csv
```

## Why Stockyard

| | Stockyard | LiteLLM | Helicone | Portkey |
|---|:---:|:---:|:---:|:---:|
| LLM proxy | ✅ | ✅ | ❌ | ✅ |
| Observability | ✅ | Basic | ✅ | ✅ |
| Hash-chain audit | ✅ | ❌ | ❌ | ❌ |
| Prompt versioning | ✅ | ❌ | ❌ | ✅ |
| Workflow engine | ✅ | ❌ | ❌ | ❌ |
| Config marketplace | ✅ | ❌ | ❌ | ❌ |
| Self-hosted binary | ✅ | Complex | ❌ | Enterprise |
| Zero dependencies | ✅ | ❌ | N/A | N/A |

## Security

- **Provider keys encrypted at rest** — AES-256-GCM with random nonce per write. Decrypted only in-memory for outbound API calls.
- **Configurable encryption key** — Set `STOCKYARD_ENCRYPTION_KEY` or let Stockyard auto-generate and persist one.
- **Stockyard API keys hashed** — SHA-256 hash stored, never the raw key. Prefix-only display.
- **Hash-chained audit ledger** — Every event cryptographically linked to the previous. Tampering breaks the chain.
- **No key leakage** — Provider keys never appear in logs, traces, API responses, or the web UI.

## Pricing

| Tier | Price | For |
|------|-------|-----|
| **Community** | Free forever | Full binary, self-hosted, all modules |
| **Individual** | $9.99/mo | Extended analytics, 5 providers |
| **Pro** | $49/mo | All modules, priority support |
| **Team** | $149/mo | Multi-user, shared dashboards |
| **Enterprise** | $499/mo | On-prem, SSO/SAML, SLA |

## Documentation

- [Getting Started](https://stockyard.dev/docs)
- [API Reference](https://stockyard.dev/docs/api)
- [Module Catalog](https://stockyard.dev/docs/modules)
- [Deployment Guide](https://stockyard.dev/docs/deploy)

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Business Source License 1.1 — see [LICENSE](LICENSE) for details. Community edition is free for all use cases.

---

<div align="center">

**[stockyard.dev](https://stockyard.dev)** · Built with 🤠 in the frontier

</div>
