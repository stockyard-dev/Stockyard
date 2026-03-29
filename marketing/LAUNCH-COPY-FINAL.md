# STOCKYARD LAUNCH — MARCH 25, 2026
# Ready to paste. All stats verified. Pricing: Free / $29 / $99 / $299.

---

# ═══════════════════════════════════════════════════════
# HACKER NEWS
# ═══════════════════════════════════════════════════════

## TITLE (paste this)

Show HN: Stockyard – Self-hosted LLM proxy, 16 apps, one Go binary

## URL

https://stockyard.dev

## FIRST COMMENT (paste immediately after submitting)

I built Stockyard because every LLM-powered app I shipped needed the same six things: proxy routing, cost tracking, caching, observability, audit trails, and safety filters. Each was a separate tool with its own Redis/Postgres/Docker setup.

Stockyard puts all of it in one Go binary with embedded SQLite:

    curl -fsSL stockyard.dev/install.sh | sh
    stockyard

Dashboard at localhost:7749/ui, proxy at localhost:7749/v1.

What's inside: 66 middleware modules (cost caps, caching, rate limiting, PII redaction, prompt injection detection, failover — all runtime-toggleable), 16 LLM providers (OpenAI, Anthropic, Gemini, Groq, Mistral, DeepSeek, Cohere, Ollama, and 8 more), 360+ API endpoints across 16 integrated apps.

Provider keys are encrypted with AES-256-GCM at rest. Audit ledger is SHA-256 hash-chained.

Middleware chain overhead: 400ns per request (benchmarked on Xeon Platinum 8581C — that's the Stockyard overhead, not the LLM latency).

Try it without installing anything: stockyard.dev/playground

MIT licensed. Self-hosted is free forever with no limits. Cloud: Pro $29/mo, Team $99/mo, Enterprise $299/mo.

Happy to answer questions about the architecture, Go+SQLite tradeoffs, or anything else.

---

## HN REPLIES (pre-written, adapt to context)

### "Why not just use LiteLLM?"

LiteLLM is great for provider abstraction — 100+ providers vs our 16. If that's your primary need, use LiteLLM.

The difference is scope. LiteLLM gives you a proxy. For observability you add Langfuse, for cost tracking another tool, audit trails another. Now you're running Python + Redis + Postgres + a SaaS.

Stockyard trades provider breadth for integration: one binary, one database, proxy + observe + trust + studio + forge + exchange all sharing state. Whether that tradeoff works depends on whether "one more tool to install" is actually your problem.

### "Benchmark numbers seem too good"

400ns is the middleware chain overhead, not total request latency. Your request still takes 1-5 seconds — that's the LLM provider. The 400ns is what Stockyard adds on top.

Methodology: `go test -bench` on Xeon Platinum 8581C, full 66-module chain with passthrough. The benchmark code is in the repo. I should make this clearer on the site — fair point.

### "This is just a bundle of existing ideas"

You're right — every individual piece exists somewhere. The value proposition is integration. Whether "one binary that does all of it" saves you enough setup and maintenance pain depends on your situation. For me running 4+ separate tools with separate databases was the bottleneck.

### "16 apps from one person — what's actually production-ready?"

Fair skepticism. The core proxy/observe/trust stack has been running on stockyard.dev with real traffic. The newer apps (governance, reputation, knowledge) have working APIs but haven't seen production traffic beyond my testing.

I'd rather ship a complete vision and iterate than hold back half the product. The code is MIT and on GitHub — you can judge what's battle-tested vs early-stage.

### "What's the catch with the free tier?"

No catch. MIT license, no telemetry, no usage limits on self-hosted. All 66 modules, all 16 providers, unlimited requests. The business model is cloud-managed tiers for teams that don't want to run infrastructure.

### "Why Go + SQLite?"

Go for the single static binary — no runtime, cross-compile for any OS, starts in 50ms. SQLite because it eliminates an entire ops category. No database to provision, configure, back up, or keep alive. WAL mode handles concurrent reads, and the proxy workload is write-light.

---

# ═══════════════════════════════════════════════════════
# PRODUCT HUNT
# ═══════════════════════════════════════════════════════

## PRODUCT NAME

Stockyard

## TAGLINE (60 chars max — this is 46)

Self-hosted LLM proxy — 16 apps, one Go binary

## DESCRIPTION (260 chars max — this is 248)

One Go binary replaces your LLM proxy, cost tracker, observability stack, audit system, prompt manager, and workflow engine. 66 middleware modules, 16 providers, embedded SQLite. Zero dependencies. MIT licensed. Self-hosted free forever.

## TOPICS

Developer Tools, Artificial Intelligence, Open Source

## PRICING

Freemium — Free (self-hosted), Pro $29/mo, Team $99/mo, Enterprise $299/mo

## URL

https://stockyard.dev

## GITHUB

https://github.com/stockyard-dev/Stockyard

---

## MAKER COMMENT (paste within 60 seconds of going live)

Hey Product Hunt! I'm Michael, solo dev behind Stockyard.

I built this because every LLM app I shipped needed the same six things — and each was a separate tool with its own database and monthly bill.

The six things:
1. Proxy routing (OpenAI, Anthropic, Gemini, Groq, + 12 more)
2. Cost tracking with hard spending caps
3. Response caching
4. Observability (traces, latency, error rates)
5. Audit trails (hash-chained, compliance-ready)
6. Safety filters (PII redaction, prompt injection detection)

Stockyard does all six in one binary:

    curl -fsSL stockyard.dev/install.sh | sh

30 seconds to your first proxied request. No Redis, no Postgres, no Docker.

Try the playground without installing anything: stockyard.dev/playground

MIT licensed. Self-hosted is free forever — all 66 modules, all 16 providers, unlimited requests.

What LLM infrastructure problems keep you up at night? I'd love to hear what to build next.

---

## PH REPLY TEMPLATES

### "How does this compare to [X]?"

Good question! The main difference is Stockyard is a single self-hosted binary vs a hosted SaaS. If you want zero-ops and don't mind a vendor dependency, [X] is solid. If you want everything local with no external dependencies, that's the gap Stockyard fills.

### "Is this production ready?"

The core proxy, observability, and audit trail are — they've been running on stockyard.dev with real traffic. The newer apps like the workflow engine and marketplace are functional but earlier stage. Code is MIT and on GitHub if you want to judge for yourself.

### "Thanks!" / "Congrats!" / "Looks great!"

Thank you! What are you building with LLMs? Always looking for real use cases to prioritize around.

### "What's on the roadmap?"

Top priorities: more provider integrations, a visual workflow builder for Forge, and better docs. What would be most useful for your stack?

### "Why Go + SQLite?"

Go for the single static binary — no runtime deps, cross-compiles to any OS, starts in 50ms. SQLite eliminates an entire ops category — no DB to provision, configure, or back up. WAL mode handles concurrency well for a proxy workload.

---

## GALLERY IMAGES (5 max, first = hero)

1. Hero — Terminal: curl install → running → first request proxied
2. Dashboard — Observe traces with cost graphs and provider breakdown
3. Playground — stockyard.dev/playground with a completed request
4. Architecture — App → Stockyard (66 modules) → 16 Providers
5. Pricing — stockyard.dev/pricing table

---

# ═══════════════════════════════════════════════════════
# LAUNCH DAY TIMELINE (Pacific Time)
# ═══════════════════════════════════════════════════════

## Monday March 24 (prep)

- [ ] Run `bash marketing/prelaunch-check.sh` — all 45 green
- [ ] Verify stockyard.dev loads, playground works, /api/apps returns 16
- [ ] Schedule PH for 12:01 AM PT Tuesday
- [ ] Have HN post + first comment in a text file ready to paste
- [ ] Have PH maker comment ready to paste

## Tuesday March 25 (launch)

12:01 AM — PH goes live
12:02 AM — Paste maker comment on PH
12:05 AM — Tweet launch announcement with PH link
 6:00 AM — Submit Show HN (peak engagement window)
 6:01 AM — Paste first comment on HN
 6:30 AM — Reddit r/golang (Go architecture angle)
 7:00 AM — Reddit r/selfhosted (zero-deps angle)
 8:00 AM — LinkedIn post
 9:00 AM — Reddit r/LocalLLaMA (Ollama angle)

ALL DAY:
- Reply to every HN comment within 30 min
- Reply to every PH comment within 1 hour
- Monitor stockyard.dev uptime + Railway logs
- Retweet anyone who mentions Stockyard

EVENING:
- Tweet metrics update (stars, upvotes, sessions)
- Post on Indie Hackers

---

# ═══════════════════════════════════════════════════════
# VERIFIED STATS (cite these confidently)
# ═══════════════════════════════════════════════════════

| Stat                  | Value      | How verified              |
|-----------------------|------------|---------------------------|
| Apps                  | 16         | /api/apps returns 16      |
| Middleware modules    | 66         | Toggle registry           |
| API endpoints         | 360+       | Route registrations       |
| LLM providers         | 16         | 17 constructors in provider/ (minus base) |
| Chain overhead        | 400ns      | go test -bench, Xeon 8581C|
| Toggle-aware chain    | 1.56µs     | go test -bench            |
| Registry lookup       | 23.1ns     | go test -bench            |
| Binary size           | ~27MB      | ls -lh bin/stockyard      |
| Startup time          | ~50ms      | Measured                  |
| Database              | SQLite WAL | Zero external deps        |
| Encryption            | AES-256-GCM| Provider keys at rest     |
| Key hashing           | SHA-256    | API keys + audit chain    |
| Password hashing      | PBKDF2 100k| Marketplace accounts      |
| License               | MIT        | LICENSE file              |
| Pricing               | Free / $29 / $99 / $299 | /pricing   |
| Test suites           | 17/17 pass | go test ./...             |
| go vet                | Clean      | 0 warnings                |
