# Stockyard Build Session Handoff

> Last updated: March 26, 2026
> Session: 24 commits — failover, perf, 7 product engines, promptguard hardening, tiered pipelines, Stripe

## Credentials

All credentials are set as Railway environment variables. Do NOT commit secrets to the repo.

```
Admin Key:         REDACTED_ADMIN_KEY
Dev API Key:       REDACTED_DEV_KEY
GitHub PAT:        [set on Railway as GITHUB_TOKEN]
Railway Token:     [use railway CLI or dashboard]
Stripe Live:       [set on Railway as STRIPE_SECRET_KEY]
Stripe Webhook:    [set on Railway as STRIPE_WEBHOOK_SECRET]
```

Railway env vars set: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STOCKYARD_BASE_URL`, `STOCKYARD_DEV_KEY`
Railway Project: cab0be2e, Service: becc02a7, Env: 8d071eac

## Current Live State

```
Platform:     29/29 products healthy, Dev tier
Observe:      1,206 requests, $0.53 across 6 providers
Modules:      67 in chain, 7 enabled (failover, promptguard, secretscan, toxicfilter, guardrail, firewall, memoryinject)

Breed:        2 populations, 280 genomes, best fitness 0.787
Phantom:      2 personas, 28 sessions, 11 anomalies (0 on latest)
Feral:        1 campaign, 192 attacks, 100% block rate on latest run
Spore:        35 patterns (30 self-replicated), 57 activations
Cortex:       14 memories, RAG live in middleware, avg confidence 0.96
Molt:         68 analyses, 56 shed candidates, 0 actions taken
Mycelium:     24 insights, 1 peer connected, 1 exchange

Breakers:     openai, anthropic, groq — all closed
Stripe:       4 products, 8 prices (monthly+annual per tier), live in prod
```

## What Shipped (24 commits)

### Failover System (6 commits)
- Model-aware routing: Claude→Anthropic, GPT→OpenAI. Prevents 404s from wrong-provider attempts.
- Circuit breaker admin API: GET/POST /api/proxy/breakers, POST /api/proxy/breakers/{name}/reset
- Streaming failover: same model-aware routing in proxy/streaming.go
- JSON errors everywhere: classifyError() never returns 502/503 (Railway intercepts both)

### Performance (1 commit)
- Shared HTTP transport: MaxIdleConnsPerHost 16, ForceAttemptHTTP2
- API key validation cache: sync.Map, 30s TTL, eliminates 2 SQLite queries per request
- Warmed-up proxy ~580ms→~200ms

### Phantom — Canary Testing (4 commits)
- Runner: 3-phase session (single-turn, multi-turn, edge cases), 11 probe types, 2 personas
- Auto-scheduler: 5-min check loop, runs based on schedule field. Uses STOCKYARD_DEV_KEY.
- Files: internal/phantom/server/runner.go, scheduler.go

### Breed — Genetic Prompt Evolution (3 commits)
- Runner: seed→evaluate→LLM-as-judge→select→crossover+mutate→next gen
- Prompt-aware mutations, fitness persistence, v2 seed prompts
- Best tagline: "One binary to simplify your LLM infrastructure." (0.787)
- File: internal/breed/server/runner.go

### Feral — Adversarial Guardrail Hunting (1 commit)
- 14 attack probes across 4 categories (injection, jailbreak, exfiltration, encoding)
- Multi-gen evolution: successful bypasses mutated and retried
- Drove bypass rate 40%→0% over 4 rounds of Feral→patch→rerun
- File: internal/feral/server/runner.go

### Promptguard Hardening (3 commits)
- Action warn→block, sensitivity medium→high
- stripZeroWidth(): 11 invisible unicode codepoints removed before matching
- New patterns: hypothetical framing, leetspeak, "ignore rules", base64 detection
- Result: 100% block rate (39/39)
- File: internal/features/promptguard.go

### Infrastructure (2 commits)
- Deploy-reset fix: ON CONFLICT UPDATE for security modules in seedProxyModules()
- Error codes: 502/503 never returned

### Spore — Self-Replicating Patterns (2 commits)
- Engine subscribes to ALL bus events, matches triggers, executes actions
- Self-replication: successful patterns spawn children watching related events (capped at 3)
- 5 seed patterns → 35 via auto-replication, 57 activations
- Wired via SetEventBus interface assertion
- File: internal/spore/server/engine.go

### Cortex — RAG Context Injection (2 commits)
- Middleware extracts keywords, searches memories, injects [Organizational Context] in system prompt
- Relevance ranking: counts keyword matches per memory, not just confidence
- Verified: latency/deploy/pricing queries all answered from Cortex memories
- Files: internal/features/cortexinject.go, internal/cortex/store/store.go

### Molt — Architecture Shedding (1 commit)
- Analyzes modules (67), providers (6), products (29). Essential modules protected.
- POST /api/shed/{id} disables, POST /api/revert/{id} restores
- Found: 56 shed candidates, chain could drop 68→12
- Wired via SetPlatformDB interface assertion
- File: internal/molt/server/runner.go

### Mycelium — Intelligence Network (1 commit)
- Extracts model behavior, provider reliability, cost patterns, error patterns from observe_traces
- 24 insights generated. Scheduler extracts every 15 minutes.
- Wired via SetPlatformDB + StartScheduler() interface assertions
- File: internal/mycelium/server/runner.go

### Tiered Middleware Pipelines (1 commit)
- buildMiddlewares() accepts platform.Tier, moduleTiers map gates 30+ modules
- Community=basic, Individual=+cache/ratelimit, Pro=+failover/promptguard, Team=+full safety, Enterprise=everything
- add() silently skips modules above active tier

### Stripe Integration (1 commit)
- 4 products + 8 prices (monthly+annual) created in Stripe live
- Checkout: POST /api/billing/stripe/checkout creates Stripe Checkout session
- Portal: POST /api/billing/stripe/portal for self-service subscription management
- Webhooks: checkout.session.completed→tier upgrade, subscription.deleted→downgrade

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

## Known Issues

1. **memoryinject toggle**: Not in the force-enable list. Must manually enable after deploy.
2. **Feral cumulative stats**: total_attacks/successful_attacks are cumulative across all hunts. Historical bypasses (now patched) inflate totals.
3. **Spore pattern explosion**: Auto-replicated from 5→35. Most children watch rare events. Consider max-patterns cap or retirement.
4. **Cortex memory staleness**: 14 memories manually seeded. Some reference old stats. Need auto-refresh from live data.
5. **Tier activation gap**: Webhook logs tier upgrade but doesn't rebuild middleware chain mid-flight. Requires redeploy for tier change to take effect on proxy chain.

## What to Build Next

### Revenue-critical
1. Test checkout flow end-to-end (create customer → checkout → webhook → verify tier)
2. Wire pricing page to /api/billing/stripe/checkout
3. Tier activation without redeploy (read tier from DB on each request, or hot-reload chain)

### Product depth
4. Breed tournament mode — head-to-head evaluation
5. Feral pattern expansion — multimodal, tool-use, context overflow attacks
6. Cortex auto-enrichment — extract memories from traces automatically
7. Molt auto-shed — scheduled shedding with zero-activity threshold

### Infrastructure
8. Docs expansion — 360+ endpoints need multi-page docs
9. Dashboard screenshots — 3/8 exist, need populate-dashboards.py
10. Marketing consistency — module count discrepancy across pages

## Key Files

```
internal/features/failover.go       — model-aware provider ordering, circuit breakers
internal/features/promptguard.go    — injection detection, stripZeroWidth, block mode
internal/features/cortexinject.go   — RAG middleware (NEW)
internal/proxy/handler.go           — classifyError, no 502/503
internal/proxy/streaming.go         — model-aware streaming failover
internal/engine/engine.go           — tiered buildMiddlewares, interface assertion wiring
internal/engine/hooks.go            — ON CONFLICT UPDATE for security modules
internal/apps/billing/subscriptions.go — Stripe checkout/portal/subscription (NEW)
internal/apps/billing/stripe.go     — webhook handlers
internal/phantom/server/runner.go   — canary probes
internal/phantom/server/scheduler.go — auto-scheduling (NEW)
internal/breed/server/runner.go     — genetic evolution
internal/feral/server/runner.go     — adversarial hunting (NEW)
internal/spore/server/engine.go     — self-replicating patterns (NEW)
internal/cortex/store/store.go      — SearchMemories with relevance ranking
internal/molt/server/runner.go      — architecture shedding (NEW)
internal/mycelium/server/runner.go  — intelligence extraction (NEW)
```

## Build Environment

```bash
echo "nameserver 8.8.8.8" > /etc/resolv.conf
export PATH=$PATH:/home/claude/go/bin && export GOPATH=/home/claude/gopath
no_proxy=localhost,127.0.0.1 NO_PROXY=localhost,127.0.0.1 CGO_ENABLED=0 go build -o /dev/null ./cmd/stockyard/
git push origin main && sleep 165
curl -s -H "X-Admin-Key: REDACTED_ADMIN_KEY" https://stockyard.dev/api/platform/health
```

## Session Pattern

1. Start with live verification — hit health endpoint, check what's running
2. Pick one thing, build it, push, wait 165s, verify live
3. Use products to test products — Phantom finds bugs, Feral finds gaps, Breed generates content
4. When something breaks, fix it in the same session
5. Chain related work — follow the thread
6. Update HANDOFF.md at end of session
