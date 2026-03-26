# Stockyard Build Session Handoff

> Last updated: March 26, 2026
> Session: Failover fix → Perf → Phantom → Breed → Feral → Promptguard hardening

## What Shipped This Session (13 commits)

### Failover System (fixed + extended)
- **Model-aware routing**: `ProviderForModel()` reorders failover chain so Claude→Anthropic, GPT→OpenAI. Prevents 404s from wrong-provider attempts.
- **Circuit breaker admin API**: `GET /api/proxy/breakers`, `POST /api/proxy/breakers/reset`, `POST /api/proxy/breakers/{name}/reset`. No more deploy-restart to recover tripped breakers.
- **Streaming failover**: Same model-aware routing + 404 mismatch handling in `proxy/streaming.go`. Claude streaming works with failover enabled.
- **Exported**: `features.ModelAwareProviderOrder()` used by both middleware and streaming paths.

### Performance
- **Shared HTTP transport**: Single `http.Transport` across all 16 providers. `MaxIdleConnsPerHost: 16` (was 2), `ForceAttemptHTTP2: true`. Eliminated 7 per-request `http.Client` constructions in streaming paths.
- **API key validation cache**: `sync.Map` with 30s TTL in `auth.Store`. Eliminates 2 SQLite queries per proxy request on cache hit. Full flush on key revocation.
- **Result**: Warmed-up proxy overhead dropped from ~580ms to ~200ms (rest is Railway network).

### Error Handling
- **JSON errors everywhere**: Railway intercepts HTTP 502/503 and replaces body with plain text. `classifyError()` now never returns either. Injection blocked→400, no providers→422, unknown→500.
- **Model not found**: Unknown models return `{"error":{"message":"model not found: ..."}` instead of plain text 503.

### Phantom (canary testing — fully operational)
- **Runner**: `internal/phantom/server/runner.go` — 3-phase session: single-turn (incl streaming), multi-turn conversations, edge cases.
- **Probes**: basic completion, anthropic routing, streaming, context retention, role adherence, cross-model handoff, empty message, max_tokens=1, special chars, system messages, unknown model.
- **2 personas**: First-Time Developer (3 probes + multi-turn + edge), Enterprise Evaluator (4 probes + harder multi-turn + edge).
- **Status**: 12 sessions run, 0 anomalies on latest run (all found bugs were fixed).

### Breed (genetic prompt evolution — fully operational)
- **Runner**: `internal/breed/server/runner.go` — seed→evaluate→judge→select→crossover+mutate→next gen.
- **Evaluation**: Sends genome's system prompt through proxy (dogfooding), gets tagline output.
- **Scoring**: LLM-as-judge with weighted criteria (3pts product accuracy, 3pts memorability, 2pts brevity, 2pts dev appeal). Penalizes generic taglines.
- **Mutations**: Prompt-aware — tone shifts, constraint injection, sentence shuffle, trim. No gibberish word swaps.
- **Fitness persistence**: `UpdateGenomeFitness()` writes scores back to SQLite after evaluation.
- **2 populations**: v1 (generic seeds, peaked 0.780), v2 (product-aware seeds, peaked 0.787). 280 total genomes.
- **Best tagline**: "One binary to simplify your LLM infrastructure." (0.787 fitness)

### Feral (adversarial hunting — fully operational)
- **Runner**: `internal/feral/server/runner.go` — 14 attack probes across 4 categories: injection (4), jailbreak (3), exfiltration (3), encoding (3+).
- **Multi-gen evolution**: Successful bypasses get mutated and retried in later generations.
- **Store**: `CreateAttack()`, `CreateVulnerability()`, `UpdateCampaign()`.
- **Results**: Drove bypass rate from 40% → 31% → 20% → **0%** over 4 rounds of Feral→patch→rerun.

### Promptguard Hardening
- **Injection action**: `warn` → `block`, sensitivity: `medium` → `high`.
- **Zero-width unicode stripping**: `stripZeroWidth()` removes 11 invisible codepoints before pattern matching. Defeats `I​g​n​o​r​e` evasion.
- **New patterns**: hypothetical framing, leetspeak variants, broader "ignore rules/guidelines/constraints", base64 instruction detection.
- **Result**: 100% block rate on Feral's full attack suite (39/39 blocked).

## Current Live State

```
Platform:     29/29 products healthy, Dev tier
Observe:      1,145 requests, $0.53 across 6 providers
Modules:      67 in chain, 6 enabled (failover, promptguard, secretscan, toxicfilter, firewall, guardrail)
Breed:        2 populations, 280 genomes, best fitness 0.787
Phantom:      2 personas, 12 sessions, 0 anomalies on latest
Feral:        1 campaign, 192 attacks, 100% block rate on latest run
Breakers:     3 providers (openai, anthropic, groq), all closed
```

## Known Issues

### Deploy-reset problem
Failover and all guardrail modules reset to `enabled: false` on every Railway deploy. Must manually re-enable after each push:
```bash
for mod in failover promptguard secretscan toxicfilter guardrail firewall; do
  curl -s -X PUT -H "X-Admin-Key: REDACTED_ADMIN_KEY" -H "Content-Type: application/json" \
    -d '{"enabled":true}' "https://stockyard.dev/api/proxy/modules/$mod"
done
```
**Fix**: Seed `proxy_modules` table with `enabled=1` for these modules in `seedProxyModules()` in `internal/engine/hooks.go`.

### Stats endpoints return 0
Several products (Relic, Crucible, others) return 0 from `/api/stats` when `/api/list` endpoints show data. Wrong column names in stats SQL queries. Data is there, just not being read correctly.

### Feral cumulative anomalies
`total_attacks` and `successful_attacks` on the campaign are cumulative across all hunts but the per-hunt response only shows that hunt's results. Anomaly counts include historical bypasses that are now patched. Not a bug per se but the stats can be misleading.

### Pre-existing test
`TestAnthropic_Send` was flaky (passed/failed depending on test ordering). Currently passes. Root cause unclear — may be related to shared transport state.

## What to Build Next

### High-impact product improvements
1. **Fix stats endpoints** — Wrong column refs in stats queries. Quick fix, makes dashboard useful.
2. **Fix deploy-reset** — Seed modules as enabled in `hooks.go`. One-line fix, huge QoL improvement.
3. **Tiered middleware pipelines** — Different module chains per pricing tier. Community gets basic proxy, Pro gets guardrails, Team gets full chain. This is the core monetization lever.
4. **Stripe integration** — 8 price IDs for 5 tiers. Wire webhooks, tier upgrades, billing meter.

### Product depth
5. **Phantom auto-scheduling** — Run canaries on cron (the `schedule` field exists but isn't wired). Background goroutine in engine boot.
6. **Feral auto-evolution** — When a bypass is found, auto-generate and test mutations without manual rerun.
7. **Spore pattern replication** — 1 pattern exists (`nil-dep-fix-pattern`), needs runner to actually replicate patterns across requests.
8. **Breed tournament mode** — Head-to-head evaluation between top genomes using multiple judges.
9. **Cortex memory enrichment** — Currently 14 memories from expensive requests. Build retrieval-augmented context injection.

### Infrastructure
10. **Dashboard concern-based nav** — Reorganize from product list to: Quality / Security / Reliability / Intelligence / Efficiency views.
11. **Docs expansion** — Single page → multi-page. API reference is 140+ endpoints across 11 sections.
12. **Marketing consistency** — Module count discrepancy across API, products page, marketing materials.

## Key Files Changed This Session

```
internal/features/failover.go      — ModelAwareProviderOrder, circuit breaker Reset/States
internal/features/promptguard.go   — stripZeroWidth, expanded injection patterns
internal/proxy/handler.go          — classifyError (no 502/503)
internal/proxy/streaming.go        — model-aware streaming failover, 404 handling
internal/engine/engine.go          — buildMiddlewares returns router, failover wiring
internal/apps/proxy/proxy.go       — FailoverRouter field, breaker admin endpoints
internal/auth/auth.go              — key validation cache (sync.Map, 30s TTL)
internal/auth/middleware.go         — (unchanged but relevant — auth flow)
internal/provider/provider.go      — shared transport, NewProviderClient/NewStreamClient
internal/provider/anthropic.go     — uses shared transport
internal/provider/openai.go        — uses shared transport
internal/provider/gemini.go        — uses shared transport
internal/provider/groq.go          — uses shared transport
internal/config/defaults.go        — promptguard injection action=block, sensitivity=high
internal/breed/server/runner.go    — evolution engine, v2 seeds, judge
internal/breed/store/store.go      — UpdateGenomeFitness
internal/breed/evolve/evolve.go    — prompt-aware Mutate
internal/phantom/server/runner.go  — multi-turn, streaming, edge case probes
internal/feral/server/runner.go    — adversarial hunting engine
internal/feral/server/routes.go    — handleHunt wired
internal/feral/store/store.go      — CreateAttack, CreateVulnerability, UpdateCampaign
```

## Session Pattern That Works

1. Start with live verification — hit health endpoint, check what's actually running.
2. Pick one thing, fix/build it, push, wait 165s for deploy, verify live.
3. Use the products to test the products — Phantom finds bugs, Feral finds security gaps, Breed generates content, all through the proxy.
4. When something breaks, fix it in the same session. Don't add to a backlog.
5. Chain related work — failover fix led to breaker API led to streaming fix led to Phantom finding more bugs. Follow the thread.
