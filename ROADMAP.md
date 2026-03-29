# Stockyard Roadmap

Last updated: March 28, 2026

This is a living document. It reflects what's shipped, what's in progress, and what's planned. No promises on dates — this is a solo project and priorities shift based on what users actually ask for.

## What's Shipped

Everything below is in the current binary. Not planned — live.

**Core proxy (Chute)**
- 40 LLM providers, auto-configured from environment variables
- 76 middleware modules (routing, caching, cost, safety, transform, validate, observe, shims)
- 400ns median proxy overhead ([benchmarks](https://stockyard.dev/benchmarks/))
- Model aliasing (Lasso) — map model names to different models/providers
- Automatic failover across providers
- Semantic and exact-match caching (no Redis required)
- Rate limiting per key, per model, with cost caps
- PII redaction, prompt injection detection, content filtering
- Prometheus `/metrics` endpoint

**Tracing & costs (Lookout)**
- Every request traced automatically — model, tokens, cost, latency, provider
- Per-model and per-provider cost dashboards
- Anomaly detection

**Audit (Brand)**
- SHA-256 hash-chained audit ledger
- Policy enforcement
- Compliance evidence export

**16 core apps** — Chute, Lookout, Brand, Tack Room, Forge, Trading Post, Billing, Team, Memory, Recall, Copilot, App Builder, Knowledge, Reputation, Governance, Marketing

**29 total products** — including Drover (autopilot routing), Lasso (replay), Feral (red-team), Phantom (persona testing), Breed (evolution), Stampede (load testing), Doubt, Verdikt, Cortex, Mycelium, Spore, Molt, Iron

**Infrastructure**
- Single ~25MB Go binary, embedded SQLite, zero external dependencies
- Docker + docker-compose + Helm chart
- Dashboard at `/ui`
- 360+ REST API endpoints
- BSL 1.1 licensed

## In Progress

Things actively being worked on right now.

- **Docs expansion** — Current docs are mostly single-page. Breaking out into per-product, per-feature pages with proper sidebar navigation.
- **Dashboard screenshots** — Only 3 of 8 marketing screenshots captured. Filling in the remaining gaps.
- **Old product name sweep** — ~8 secondary pages still reference pre-rename product names in body copy.

## Planned (Near-Term)

Things that will likely happen in the next few weeks. No hard dates.

- **Model router UI** — Visual drag-and-drop routing configuration in the dashboard.
- **Request replay export** — Export Lasso traces as curl commands or Postman collections.
- **SDK starters** — Go, Python, and TypeScript SDK skeletons for the API.
- **Playground share backend** — `POST /api/playground/share` storing sessions in SQLite with short IDs. Currently falls back to base64 URL hashing.

## Planned (Medium-Term)

Things on the radar but not actively in development.

- **Multi-page docs site** — Full sidebar navigation, search, per-product documentation.
- **Terraform / Pulumi modules** — Infrastructure-as-code for Stockyard deployments.
- **Audit ledger export** — Parquet and signed JSONL export formats.
- **VS Code extension** — Provider config and trace viewing from the editor.

## Not Planned (Yet)

Things people have asked about that aren't on the roadmap. This doesn't mean "never" — it means there's no plan right now.

- **Horizontal scaling / multi-writer** — SQLite is single-writer. For most deployments this is fine. If you need >1K concurrent writes per second, Stockyard may not be the right fit today. Litestream replication is a possible path but not being actively worked on.
- **SOC 2 / HIPAA certification** — Real certifications require real audits. This is a solo project. If enterprise compliance is a hard requirement today, Stockyard is honest about not having it.
- **SSO / SAML** — Planned for the Enterprise tier but not yet built. The [security page](https://stockyard.dev/security/) is explicit about this.
- **Managed multi-tenant hosting** — Focus is on self-hosted. The binary is the product.

## How to Influence This Roadmap

Open an issue on [GitHub](https://github.com/stockyard-dev/Stockyard/issues), post in [r/selfhosted](https://reddit.com/r/selfhosted), or email michael@stockyard.dev. The features that get built are the ones real users ask for.
