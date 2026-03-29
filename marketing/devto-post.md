---
title: I replaced 6 LLM tools with one Go binary
published: true
tags: go, opensource, ai, devtools
canonical_url: https://stockyard.dev/blog/why-i-built-stockyard/
---

Every LLM app I built needed the same six things. Each was a separate tool with its own database.

**The six things:**
1. Proxy routing across providers (OpenAI, Anthropic, Gemini, etc.)
2. Cost tracking with hard spending caps
3. Response caching
4. Observability (traces, latency, error rates)
5. Audit trails for compliance
6. Safety filters (PII redaction, prompt injection detection)

To get all six, I was running LiteLLM + Langfuse + custom middleware + Postgres + Redis. Five services, three databases, Docker Compose, and a monthly SaaS bill.

So I built Stockyard. One Go binary, embedded SQLite, zero external dependencies.

```bash
curl -fsSL stockyard.dev/install.sh | sh
stockyard
```

Dashboard at `localhost:7749/ui`. Proxy at `localhost:7749/v1`. Done.

## What's inside

**66 middleware modules** that you can toggle at runtime via API call. Cost caps, caching, rate limiting, PII redaction, prompt injection detection, provider failover, content filtering, and 59 more. Enable or disable any of them without restarting.

**16 LLM providers.** OpenAI, Anthropic, Gemini, Groq, Mistral, DeepSeek, Cohere, Ollama, and 8 more. OpenAI-compatible API, so you just change your base URL.

**16 integrated apps.** Proxy, Observe, Trust, Studio, Forge, Exchange, Billing, Team, and 8 more. They all share one SQLite database, so your traces, costs, audit records, and prompt experiments are in one place.

## Why Go + SQLite

Go compiles to a single static binary. No runtime, no interpreter, no node_modules. Cross-compiles to every OS. Starts in 50ms.

SQLite eliminates an entire ops category. No database to provision, configure, back up, or keep alive. WAL mode handles concurrent reads. The proxy workload is write-light, so SQLite is a perfect fit.

The result: a ~27MB binary that replaces a stack that used to need Docker Compose.

## Performance

The full 66-module middleware chain adds 400ns per request. That's the Stockyard overhead, not the LLM latency. Your actual request time is still dominated by the provider (1-5 seconds).

Benchmarked on Xeon Platinum 8581C with `go test -bench`. The benchmark code is in the repo.

## Try it

**Playground (no install):** [stockyard.dev/playground](https://stockyard.dev/playground)

**Install:**
```bash
curl -fsSL stockyard.dev/install.sh | sh
```

**Source:** [github.com/stockyard-dev/Stockyard](https://github.com/stockyard-dev/Stockyard)

MIT licensed. Self-hosted is free forever with no limits. 76K lines of Go.

What LLM infrastructure problems are you dealing with? I'd love to hear what to build next.
