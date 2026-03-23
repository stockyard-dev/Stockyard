# Stockyard Launch Copy — March 25, 2026
## Product Hunt + Hacker News — Final, Launch-Ready

Current stats: 16 apps, 66 modules, 360+ endpoints, 16 provider integrations
Pricing: Community (free), Individual ($9.99), Pro ($49), Team ($149), Enterprise ($499)

---

# HACKER NEWS

## Title (80 char limit)

```
Show HN: Stockyard – Self-hosted LLM proxy, 16 apps, one Go binary
```

## First Comment (post immediately — this is your pitch)

```
I built Stockyard because every LLM-powered app I shipped needed the same 
six things: proxy routing, cost tracking, caching, observability, audit 
trails, and safety filters. Each was a separate tool with its own 
Redis/Postgres/Docker setup.

Stockyard puts all of it in one Go binary with embedded SQLite:

  curl -fsSL stockyard.dev/install.sh | sh
  stockyard

That's it. Dashboard at localhost:4200/ui, proxy at localhost:4200/v1.

What's inside:
- 66 middleware modules (cost caps, caching, rate limiting, PII redaction, 
  prompt injection detection, failover, etc.) — all runtime-toggleable
- 16 LLM providers (OpenAI, Anthropic, Gemini, Groq, Ollama, etc.)
- 360+ API endpoints across 16 integrated apps
- AES-256-GCM encryption for provider keys at rest
- SHA-256 hash-chained audit ledger for compliance

Overhead: 400ns per request through the full 66-module chain (benchmarked 
on Xeon Platinum 8581C).

Try it without installing: stockyard.dev/playground

MIT licensed. Self-hosted is free forever with no limits. Source:
github.com/stockyard-dev/Stockyard

Happy to answer questions about the architecture, Go+SQLite choices, 
or anything else.
```

## Anticipated Questions & Answers

### "Why not just use LiteLLM?"

```
LiteLLM is great for provider abstraction — it supports 100+ providers vs 
our 16. If that's your main need, use LiteLLM.

The difference is scope. LiteLLM gives you a proxy. To get observability 
you add Langfuse. Cost tracking, another tool. Audit trails, another. Now 
you're running Python + Redis + Postgres + a SaaS dependency.

Stockyard trades provider breadth for integrated simplicity: one binary, 
one database, proxy + observe + trust + studio + forge + exchange all 
talking to each other. Whether that tradeoff works depends on your stack.
```

### "The benchmark numbers seem too good"

```
400ns is the middleware chain overhead, not total latency. Your actual 
request time is still dominated by the LLM provider (1-5 seconds). 
The 400ns is just what Stockyard adds.

Benchmark code is in the repo. `go test -bench` on a Xeon Platinum 8581C,
full 66-module chain with a passthrough request. Reproducible.

Should probably make this distinction clearer on the benchmarks page — 
fair point.
```

### "This is just a bundle of existing ideas"

```
You're right — every individual piece exists somewhere. The value is 
integration. Right now getting proxy routing + cost tracking + 
observability + audit trails means running 4+ services with separate 
databases, configs, and failure modes.

Whether "one binary that does all of it" is worth anything depends on 
whether integration pain is a real problem for you. For me it was.
```

### "16 apps seems like a lot for one person"

```
Fair skepticism. The core proxy/observe/trust stack is battle-tested — 
it's been running on stockyard.dev with real traffic. The newer apps 
(governance, reputation, knowledge, etc.) are functional with working 
APIs but haven't seen production traffic beyond my testing.

I'd rather ship a complete vision and iterate than hold back half the 
product. The code is MIT — you can read it and judge what's production-
ready vs early-stage.
```

### "What's the catch with free tier?"

```
No catch. MIT license, no telemetry, no usage limits on self-hosted. 
The business model is the managed cloud tiers for teams that don't want 
to run infrastructure. Individual $9.99/mo, Pro $49/mo, Team $149/mo.
```

---

# PRODUCT HUNT

## Listing Details

**Product name:** Stockyard

**Tagline (60 char max):**
```
Self-hosted LLM proxy — 16 apps, one Go binary
```

**Description (260 char max):**
```
One Go binary replaces your LLM proxy, cost tracker, observability stack, audit 
system, prompt manager, and workflow engine. 66 middleware modules, 16 providers, 
embedded SQLite. Zero dependencies. MIT licensed. Self-hosted free forever.
```

**Topics:** Developer Tools, Artificial Intelligence, Open Source

**Pricing:** Freemium — Free (self-hosted), Individual $9.99/mo, Pro $49/mo, Team $149/mo, Enterprise $499/mo

**URL:** https://stockyard.dev

**GitHub:** https://github.com/stockyard-dev/Stockyard

---

## Maker Comment (post within 60 seconds of launch going live)

```
Hey Product Hunt! I'm Michael, solo founder of Stockyard.

I built this because every LLM app I shipped needed the same six things — 
and each one was a separate tool with its own database, Docker setup, and 
monthly bill.

The six things:
1. Proxy routing (route to OpenAI, Anthropic, Gemini, etc.)
2. Cost tracking (know what you're spending before the invoice)
3. Caching (stop paying for the same answer twice)
4. Observability (traces, latency, error rates)
5. Audit trails (who asked what, when, compliance)
6. Safety filters (PII redaction, prompt injection detection)

Stockyard does all six in one binary. curl install, 30 seconds to your 
first proxied request. No Redis. No Postgres. No Docker.

Try the playground without installing anything: stockyard.dev/playground

The whole thing is MIT licensed. Self-hosted is free forever with no 
limits — all 66 modules, all 16 providers, unlimited requests.

What LLM infrastructure headaches are you dealing with? I'd love to 
hear what to build next.
```

---

## Gallery Images Needed (5 max, first is hero)

1. **Hero** — Terminal showing `curl install` → stockyard running → first request proxied. Dark theme, clean typography.
2. **Dashboard** — Observe traces with live cost data, latency graphs, provider breakdown.
3. **Playground** — The web playground at stockyard.dev/playground with a completed request.
4. **Architecture** — Clean diagram: App → Stockyard (66 modules) → Providers. Show the middleware chain.
5. **Pricing** — The 5-tier pricing table from stockyard.dev/pricing.

---

## First 5 Reply Templates

### To "How does this compare to [competitor]?"
```
Good question! The main difference is that Stockyard is a single self-hosted 
binary vs a hosted SaaS. If you want zero-ops and don't mind a vendor 
dependency, [competitor] is solid. If you want everything local with no 
external dependencies, that's the gap Stockyard fills.
```

### To "Is this production ready?"
```
The core proxy, observability, and audit trail are — they've been running on 
stockyard.dev with real traffic. The newer apps (workflow engine, marketplace) 
are functional but earlier stage. The code is MIT and on GitHub if you want to 
judge for yourself!
```

### To "Nice work!" / "Congrats!"
```
Thank you! Would love to know what you're building with LLMs — always looking 
for real use cases to prioritize features around.
```

### To "What's on the roadmap?"
```
Top priorities: more provider integrations (currently 16), a visual workflow 
builder for Forge, and better streaming token accuracy. What would be most 
useful for your stack?
```

### To "Why Go + SQLite?"
```
Go for the single static binary — no runtime dependencies, cross-compile for 
any OS, starts in 50ms. SQLite because it eliminates an entire ops category — 
no database to provision, configure, back up, or keep alive. WAL mode handles 
concurrent reads well, and the proxy workload is write-light.
```

---

# LAUNCH DAY TIMELINE (PT)

## Monday March 24 (prep day)

- [ ] Run `bash marketing/prelaunch-check.sh` — all 45 green
- [ ] Verify stockyard.dev loads, playground works, /api/apps returns 16
- [ ] Schedule PH listing for 12:01 AM PT Tuesday
- [ ] Draft HN post in a text file (don't submit yet)
- [ ] Pre-write 5 tweets in Twitter draft
- [ ] Have maker comment ready to paste (clipboard or pinned note)

## Tuesday March 25 (launch day)

**12:01 AM PT** — PH goes live automatically
**12:02 AM PT** — Post maker comment on PH
**12:05 AM PT** — Tweet 1 (launch announcement) with PH link
**6:00 AM PT** — Submit Show HN (HN peaks 6-9 AM PT)
**6:01 AM PT** — Post first comment on HN
**6:30 AM PT** — Reddit r/golang (architecture angle)
**7:00 AM PT** — Reddit r/selfhosted (zero-deps angle)
**8:00 AM PT** — LinkedIn post
**9:00 AM PT** — Reddit r/LocalLLaMA (Ollama support angle)

**All day:**
- Reply to every PH comment within 1 hour
- Reply to every HN comment within 30 minutes (HN rewards engagement)
- Retweet/share anyone who mentions Stockyard
- Monitor stockyard.dev uptime and Railway logs

**Evening:**
- Tweet metrics update (stars, upvotes, playground sessions)
- Post Indie Hackers launch post

---

# QUICK REFERENCE — STATS TO CITE

| Stat | Value | Source |
|------|-------|--------|
| Apps | 16 | `/api/apps` returns 16 |
| Middleware modules | 66 | Toggle registry |
| API endpoints | 360+ | Route registrations |
| LLM providers | 16 | Provider package |
| Chain overhead | 400ns | `go test -bench` |
| Toggle-aware chain | 1.56µs | `go test -bench` |
| Registry lookup | 23.1ns | `go test -bench` |
| Binary size | ~15MB | `ls -la bin/stockyard` |
| Startup time | ~50ms | Measured |
| Database | SQLite (WAL mode) | Zero-ops |
| Encryption | AES-256-GCM | Provider keys at rest |
| License | MIT | LICENSE file |
| Pricing | Free → $9.99 → $49 → $149 → $499 | /pricing |
