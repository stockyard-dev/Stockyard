# HN Comment Prep — Honest Answers to Hard Questions

Use these as starting points, not scripts. Adapt to the specific comment. Be brief — HN rewards concision.

---

## "This is just a bundle of existing ideas."

**Answer:**
You're right — every individual piece exists somewhere. The value is integration. Right now if you want proxy routing, cost tracking, observability, and audit trails, you're running LiteLLM + Langfuse + your own Postgres + some custom middleware. Each has its own setup, its own failure modes, its own config.

Stockyard puts all of that in one binary with one SQLite database. No orchestration, no Docker compose, no separate services to keep alive. Whether that tradeoff is worth it depends on whether integration pain is a real problem for you. For me it was.

---

## "Why should I trust ops/compliance claims from a new project?"

**Answer:**
Fair question. The audit ledger is SHA-256 hash-chained — each event includes the hash of the previous event, so any tampering breaks the chain. You can verify it yourself:

```
curl localhost:7749/api/trust/ledger/verify
```

The code is in `internal/features/compliancelog.go` if you want to inspect the implementation. Provider keys are encrypted with AES-256-GCM at rest (`internal/auth/crypto.go`).

I won't claim this replaces a SOC 2 audit. But it gives you a tamper-evident log of every LLM interaction, which is more than most teams have today.

---

## "Why not LiteLLM + Helicone + my own DB?"

**Answer:**
That stack works. The question is whether you want to run three services or one.

LiteLLM gives you better provider coverage (100+ vs 16). Helicone gives you a polished hosted dashboard. But now you're running Python + Redis + Postgres + a SaaS dependency, and your audit trail lives in someone else's infrastructure.

Stockyard trades breadth for simplicity: one binary, one database, everything local. If you need 100 providers, use LiteLLM. If you want integrated local control with zero ops, that's the gap Stockyard fills.

---

## "Why is cloud emphasized if the wedge is self-hosted?"

**Answer:**
It shouldn't be — and I've been fixing this. The primary path is self-hosted: `curl install | sh`, run the binary, done. Cloud exists for teams that want managed hosting, but the product is designed self-hosted-first.

If the site still feels cloud-forward when you looked, I'd appreciate knowing where so I can fix it.

---

## "What is actually production-ready versus aspirational?"

**Answer:**
Production-ready today:
- Proxy with all 66 middleware modules (tested, benchmarked at 400ns/request)
- Observe (tracing, cost dashboards — running on the live site right now with real data)
- Trust (hash-chained audit ledger, policy enforcement)
- 16 provider integrations
- Billing with Stripe integration, Team with RBAC

Functional and complete but newer:
- Studio (prompt templates and A/B experiments work, but the experiment runner is basic)
- Forge (DAG workflows execute correctly, but the visual builder is early)
- Exchange (packs install and work, but the marketplace has only first-party packs so far)
- Memory, Recall, Copilot, App Builder, Knowledge, Reputation, Governance, Marketing — all built and deployed, APIs work, but they haven't seen production traffic beyond my own testing yet

I won't pretend all 16 apps are battle-tested at scale. The core proxy/observe/trust stack is. The rest is functional and improving. You can read the code and judge for yourself.

---

## "What's the license?"

**Answer:**
MIT. Full stop. Free to use, modify, and distribute. The LICENSE file is in the repo root.

The business model is the hosted/managed tiers, not licensing restrictions.

---

## "The benchmark numbers seem too good."

**Answer:**
The 400ns number is the middleware chain overhead — not total request latency. Your actual request latency is dominated by the LLM provider (typically 1-5 seconds). The 400ns is just what Stockyard adds on top.

Benchmark methodology: `go test -bench` on a Xeon Platinum 8581C, testing the full 58-module chain execution with a passthrough request. The 1.56µs toggle chain number is for hot-path module enable/disable. Results are reproducible — the benchmark code is in the repo.

I should probably make this clearer on the benchmarks page.

---

## General principles for HN comments:

1. **Lead with agreement** — "You're right" or "Fair question" before explaining.
2. **Concede real weaknesses** — 16 providers vs 100+, early-stage ecosystem, Studio/Forge are newer.
3. **License is MIT** — mention it if asked. It removes a common objection.
4. **Link to code** — specific file paths build credibility (`internal/features/compliancelog.go`).
5. **Be brief** — 3-5 sentences max per reply. HN hates walls of text.
6. **Don't argue** — if someone prefers LiteLLM, say "makes sense for your use case" and move on.
