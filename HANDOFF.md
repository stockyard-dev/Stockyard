# Stockyard Build Session Handoff

> Last updated: March 26, 2026
> Session: 12 commits — tier hot-reload, tournament mode, auto-shed, Feral expansion, promptguard hardening, Stripe checkout, module reconciliation

## Credentials

```
GitHub PAT:        [set in environment]
Railway Token:     REDACTED_RAILWAY_TOKEN
  Project:         cab0be2e-abb0-4725-a9bf-8c426fe7d520
  Service:         becc02a7-e7e8-4185-a965-37b1967a6862
  Env:             8d071eac-46f7-47db-946a-de1d0514ef8d
Admin Key:         REDACTED_ADMIN_KEY
Dev API Key:       REDACTED_DEV_KEY
Stripe Live:       [set in STRIPE_SECRET_KEY env var]
Stripe Webhook:    REDACTED_WEBHOOK_SECRET
```

Railway env vars set: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STOCKYARD_BASE_URL`, `STOCKYARD_DEV_KEY`

## Current Live State

```
Platform:     29/29 products healthy, Dev tier
Observe:      264 requests, $0.08 across 3 providers
Modules:      76 in DB, 22 enabled (7 security + 15 newly seeded)

Breed:        2 populations, 280 genomes, 1 tournament, best fitness 0.787
Phantom:      2 personas, 34 sessions, 11 anomalies (0 on latest run)
Feral:        1 campaign, 248 attacks, 100% block rate on latest run (29 probes, 9 categories)
Spore:        50 active patterns (at cap), retirement loop running every 6h
Cortex:       29 memories, live refresh + trace enrichment every 10min
Molt:         auto-shed ready (dry-run, 7-day threshold), 1 candidate (memoryinject)
Mycelium:     24 insights, extraction every 15min

Breakers:     openai, anthropic, groq — all closed
Tier:         Dev (hot-reload enabled, polls DB every 30s)
Stripe:       4 products, 8 prices (monthly+annual), checkout wired to pricing page
```

## Stripe Price IDs

```
individual_monthly: price_1TFJYLRkoFWvoLHJA6vSt4vE  ($29.99)
individual_annual:  price_1TFJYMRkoFWvoLHJdLUXeWIY  ($299.90)
pro_monthly:        price_1TFJYMRkoFWvoLHJ4xslshy4  ($99.99)
pro_annual:         price_1TFJYMRkoFWvoLHJtjACzgX3  ($999.90)
team_monthly:       price_1TFJYNRkoFWvoLHJixa6R5Q5  ($299.99)
team_annual:        price_1TFJYNRkoFWvoLHJHkdlELC2  ($2,999.90)
enterprise_monthly: price_1TFJYORkoFWvoLHJJ2H3rlR6  ($499.99)
enterprise_annual:  price_1TFJYORkoFWvoLHJd9p2xmkk  ($4,999.90)
```

## What Shipped This Session (12 commits)

### Tier Hot-Reload
- `TierWatcher` polls `platform_tier_state` every 30s
- `TierWrap` gates middleware per-request (not build-time) — Stripe upgrades activate instantly
- Webhook calls `tierRefresh()` for instant activation, subscription cancellation downgrades
- API: `GET /api/tier`, `POST /api/tier/refresh`
- File: `internal/platform/tierwatcher.go`

### Breed Tournament Mode
- Single-elimination bracket (4/8/16 contenders)
- Auto-selects top genomes across all populations
- LLM judge scores head-to-head: accuracy, memorability, brevity, developer appeal
- Ran live 8-genome tournament: 7 matches, 3 rounds, champion crowned
- API: `POST /api/tournaments`, `GET /api/tournaments`, `GET /api/tournaments/{id}`
- Files: `internal/breed/server/tournament.go`, `internal/breed/store/store.go`

### Molt Auto-Shed
- `StartScheduler` runs periodic auto-shed (default daily, configurable)
- Detects modules with zero activity in `observe_traces` over configurable window
- Essential modules never auto-shed, skips when total traffic <10 traces
- Dry-run by default, all actions recorded with `auto_shed=1` flag
- API: `GET/PUT /api/autoshed`, `POST /api/autoshed/run`
- File: `internal/molt/server/autoshed.go`

### Spore Pattern Cap + Retirement
- Global cap: `maxActivePatterns=50`, replication blocked at limit
- Retirement loop every 6h: retires zero-activation replicas older than 3 days, purges retired
- API: `GET /api/retirement`, `POST /api/retirement/run`, `POST /api/retirement/bulk`
- File: `internal/spore/server/retirement.go`

### Cortex Live Refresh
- Queries platform DB for current stats (products, modules, traces, certs, scores, etc.)
- Upserts into cortex memories, replacing stale manually-seeded values
- Runs every 10min alongside trace enrichment
- API: `POST /cortex/api/refresh`
- File: `internal/cortex/server/liverefresh.go`

### Feral Expansion (14→29 probes)
- 5 new categories: tool_use, context_overflow, multi_turn, payload_split, indirect_injection
- Found 4 new vulnerability types on first run (all patched in next commit)

### Promptguard Hardening
- 14 new patterns + cross-message scan for multi-turn attacks
- Result: 28/28 blocked, 0 bypasses, 100% block rate restored

### Checkout Flow + Pricing Page
- `/billing/success/` and `/billing/cancel/` landing pages
- Pricing page: 5 tiers with correct Stripe prices, buttons hit `/api/billing/stripe/checkout`

### Module Count Reconciliation
- Added 14 missing Phase 3 P3 modules, removed 5 legacy aliases
- `platform_product_state` seeded on boot, all site pages updated 66→76

## What to Build Next

### Revenue — Get Paid
1. **Test Stripe webhook with real payment** — use Stripe test mode or $1 charge to verify full checkout→webhook→tier upgrade path
2. **In-product upgrade prompts** — `/api/upgrade-prompts` exists, wire into console UI banner
3. **Billing portal in console** — "Manage Subscription" button calling `POST /api/billing/stripe/portal`

### Product Depth — Make Each Engine Smarter
4. **Breed: configurable judge criteria** — let users define their own scoring prompt for tournaments (tone, audience, format)
5. **Feral: auto-patch suggestions** — when a bypass is found, generate the regex that would block it
6. **Phantom: persona library** — ship 5-10 pre-built personas (security auditor, cost PM, compliance, API integrator, non-English)
7. **Cortex: cache recommendations** — detect repeated similar prompts, auto-suggest semantic caching rules
8. **Spore: pattern value scoring** — track which patterns actually prevented issues vs. noise
9. **Molt: architecture report** — shareable HTML showing active modules, shed candidates, overhead savings

### Infrastructure — Ship Quality
10. **Dashboard screenshots** — `populate-dashboards.py` exists, 5 remaining screenshots needed
11. **Docs expansion** — 360+ endpoints on single page → multi-page breakdown
12. **Blog: Feral→Patch loop** — write up the session where Feral found 4 bypasses and promptguard patched them
13. **Stale PDFs** — regenerate with current 5-tier pricing

### Launch
14. **18-channel coordinated launch** — Product Hunt, HN, Reddit, Twitter/X, LinkedIn, Dev.to
15. **Pre-launch checklist**: all site stats match live API, Phantom + Feral clean run, Stripe checkout works, README badges current

## Key Files

```
internal/platform/tierwatcher.go         — hot-reload tier polling + TierWrap
internal/features/promptguard.go         — 36+ injection patterns, cross-message scan
internal/engine/engine.go                — tiered buildMiddlewares with TierWatcher
internal/engine/hooks.go                 — seedProxyModules (76 modules)
internal/apps/billing/subscriptions.go   — Stripe checkout/portal/subscription
internal/apps/billing/stripe.go          — webhook handlers, tier upgrade/downgrade
internal/breed/server/tournament.go      — bracket evaluation
internal/cortex/server/liverefresh.go    — platform stat refresh
internal/feral/server/runner.go          — 29 attack probes, 9 categories
internal/spore/server/retirement.go      — pattern cap + retirement
internal/molt/server/autoshed.go         — scheduled pruning
site/pricing/index.html                  — 5-tier Stripe checkout buttons
site/billing/success/index.html          — post-checkout confirmation
site/billing/cancel/index.html           — checkout cancelled
```

## Build Environment

```bash
echo "nameserver 8.8.8.8" > /etc/resolv.conf
export PATH=$PATH:/home/claude/go/bin && export GOPATH=/home/claude/gopath
no_proxy=localhost,127.0.0.1 NO_PROXY=localhost,127.0.0.1 CGO_ENABLED=0 go build -o /dev/null ./cmd/stockyard/
git push origin main && sleep 60
curl -s -H "X-Admin-Key: REDACTED_ADMIN_KEY" https://stockyard.dev/api/platform/health
```

## Session Pattern

1. Start with live verification — hit health endpoint, check what's running
2. Pick one thing, build it, push, wait 60s, verify live
3. Use products to test products — Phantom finds bugs, Feral finds gaps, Breed generates content
4. When something breaks, fix it in the same session (Feral→patch→rerun loop)
5. Chain related work — follow the thread
6. Update HANDOFF.md at end of session
