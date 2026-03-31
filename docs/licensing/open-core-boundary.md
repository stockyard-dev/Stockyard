# Open-Core Boundary: Stockyard Proxy (OSS) vs Commercial Platform (BSL)

## Goal

Split Stockyard into two layers:

1. **Stockyard Proxy** — open source (Apache 2.0), standalone binary
2. **Stockyard Platform** — commercial (BSL 1.1), full binary with all apps and products

The OSS proxy must be a real, usable product. The commercial platform must retain the features that justify paid tiers.

## Package Classification

### OSS_PROXY_CORE (Apache 2.0)

| Package | Purpose |
|---|---|
| `internal/proxy` | Core reverse proxy, request handler, streaming |
| `internal/provider` | Provider adapters (OpenAI, Anthropic, Gemini, Groq, etc.) |
| `internal/toggle` | Middleware runtime enable/disable registry |
| `internal/config` | Configuration loading and defaults |
| `internal/storage` | SQLite DB, migrations, request logging, spend tracking |
| `internal/auth` | API key auth and middleware |
| `internal/billingerr` | Error type definitions |
| `internal/tracker` | Per-request spend counter |
| `internal/slog` | Structured logging |
| `internal/features` (subset) | Core middleware modules (see below) |
| `cmd/stockyard-proxy` | OSS binary entry point |

### OSS Middleware Modules

These modules ship with the open-source proxy (Community tier, no license gate):

**Routing:** modelalias, failover, abrouter, localsync, modelswitch, geminishim, anthrofit
**Caching:** cache, embedcache
**Cost:** spend, caps, costwarn
**Reliability:** retry, retrypilot, ratelimit, ipfence, idlekill
**Observability:** logging, responseheaders, devproxy
**Transform:** tokentrim, promptslim, outputcap, contextwindow

All modules live in `internal/features/`. The OSS binary enables the subset above. The code for all modules remains in the same package — the boundary is enforced by what `BootProxy()` enables, not by moving files.

### BSL_COMMERCIAL (BSL 1.1)

| Package | Purpose |
|---|---|
| `internal/apps/observe` | Advanced observability, dashboards, anomaly detection |
| `internal/apps/trust` | Hash-chained audit ledger, compliance, evidence packs |
| `internal/apps/studio` | Prompt templates, A/B experiments, benchmarks |
| `internal/apps/forge` | DAG workflow engine, tool registry |
| `internal/apps/exchange` | Config packs, marketplace |
| `internal/apps/billing` | Stripe integration, metering |
| `internal/apps/team` | Team management, isolation |
| `internal/apps/*` (remaining) | Marketing, governance, copilot, etc. |
| `internal/dashboard` | Admin dashboard SPA |
| `internal/platform` | Product mount system, tier gating |
| All 29 products | Bid, Breed, Cortex, Doubt, Echo, Feral, etc. |
| `internal/engine` (Boot function) | Full platform bootstrap |
| `internal/license` | License enforcement |
| `internal/mcp` | MCP server |
| `internal/features` (subset) | Advanced middleware: promptguard, toxicfilter, guardrail, hallucicheck, compliancelog, secretscan, agentguard, codefence, billingmeter, tenantwall, etc. |
| `cmd/stockyard` | Commercial binary entry point |

### SHARED_INTERNAL

| Package | Status |
|---|---|
| `internal/engine` | Contains both `Boot()` (commercial) and `BootProxy()` (OSS). Both coexist in the same package. |
| `internal/features` | Contains both OSS and BSL modules. Separation is by boot path, not file location. |

## Rationale

### Why not move packages into separate directories?

The Go `internal/` convention already prevents external import. Moving packages would:
- Break all import paths across 200+ files
- Require updating every test
- Risk merge conflicts on every branch
- Add no real protection beyond what the license provides

Instead, the boundary is enforced by:
1. **Two separate `main.go` files** that import different subsets
2. **License headers on source files** marking OSS vs BSL
3. **The `BootProxy()` function** that only wires OSS components

### Why include caching and failover in OSS?

Without caching and failover, the OSS proxy is just a passthrough — nobody would use it. These features make the proxy genuinely useful and create the adoption path that leads to commercial upgrades.

### Why keep the dashboard BSL?

The dashboard is the primary conversion surface. Users who outgrow the proxy API and want a visual interface upgrade to the commercial binary.

## Risk Areas

1. **`internal/features` is a flat package** — OSS and BSL modules coexist. A contributor could accidentally import a BSL module into OSS boot path. Mitigated by code review and CI checks.

2. **`engine.go` is 2760 lines** — `BootProxy()` must be carefully extracted to avoid duplicating logic. Shared helpers remain in engine.go and are used by both boot paths.

3. **Storage migrations** — OSS needs a subset of migrations. The migration system must not create tables for BSL features in the OSS binary.

## Migration Sequence

1. Create `engine.BootProxy()` with minimal boot path
2. Create `cmd/stockyard-proxy/main.go`
3. Add Apache 2.0 LICENSE for proxy, keep BSL for platform
4. Add license headers to source files
5. Update README with dual-license explanation
6. Add Dockerfile for OSS binary
7. Update build pipeline to produce both binaries

## Unresolved

- **MCP server**: Currently classified as BSL. Could move to OSS if PM decides it increases adoption enough to justify.
- **Basic trace API**: OSS includes request logging via the `logging` middleware. Whether to expose a `/api/traces` read endpoint (no dashboard) is a PM call.
