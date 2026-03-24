# Newsletter & Podcast Pitches — Ready to Send

---

## Changelog (changelog.com)

**Subject:** Stockyard: self-hosted LLM proxy, one Go binary, MIT licensed

Hey Changelog team,

I just open-sourced Stockyard, a self-hosted LLM proxy and control plane that ships as a single Go binary with zero external dependencies.

It replaces the LiteLLM + Langfuse + Redis + Postgres stack with one ~27MB binary and embedded SQLite. 66 middleware modules (cost caps, caching, rate limiting, PII redaction, failover), 16 LLM providers, 400ns chain overhead.

76K lines of Go, MIT licensed, free forever self-hosted.

GitHub: https://github.com/stockyard-dev/Stockyard
Site: https://stockyard.dev
Playground: https://stockyard.dev/playground

Happy to come on the show or do a written interview about the Go + SQLite architecture decisions.

Michael
stockyard.dev

---

## Go Time (gotime.fm)

**Subject:** One Go binary replaces 6 LLM infrastructure tools

Hi,

I built Stockyard, a 76K-line Go project that ships as a single static binary (no CGO) with embedded SQLite. It's a complete LLM proxy and control plane: 66 middleware modules, 16 providers, observability, audit trails, prompt management.

The architecture might make an interesting episode: how to build a zero-dependency platform in Go, when SQLite beats Postgres, and designing a hot-swappable middleware chain that adds 400ns per request.

GitHub: https://github.com/stockyard-dev/Stockyard
Benchmarks: https://stockyard.dev/benchmarks/

Michael
stockyard.dev

---

## TLDR Newsletter (tldr.tech/submit)

**Title:** Stockyard: Self-Hosted LLM Proxy in One Go Binary

**Body:** Stockyard ships 66 middleware modules, 16 LLM providers, observability, and audit trails as a single Go binary with embedded SQLite. Zero external dependencies. MIT licensed. Free forever self-hosted.

**URL:** https://github.com/stockyard-dev/Stockyard

---

## Console Newsletter (console.dev)

**Name:** Stockyard
**URL:** https://github.com/stockyard-dev/Stockyard
**Description:** Self-hosted LLM proxy and control plane. 66 middleware modules, 16 providers, embedded SQLite, one Go binary. Replaces LiteLLM + Langfuse + Redis + Postgres with zero external dependencies. MIT licensed.
**What makes it interesting:** Ships as a single ~27MB static binary. No Docker required. 400ns middleware chain overhead. The entire platform runs on embedded SQLite with WAL mode.

---

## Hacker Newsletter (hackernewsletter.com)

No submission needed. If the Show HN hits the front page, they'll pick it up automatically. They curate from HN weekly.

---

## This Week in Rust / Go Weekly / Golang Weekly

**Go Weekly** (golangweekly.com) - Submit via their form or email
**Subject:** Stockyard: 76K-line Go project, single binary LLM proxy

Self-hosted LLM proxy and control plane. 66 runtime-toggleable middleware modules, 16 providers, embedded SQLite. Single static binary, no CGO, MIT licensed. 400ns per-request chain overhead benchmarked on Xeon Platinum 8581C.

https://github.com/stockyard-dev/Stockyard

---

## DevOps Weekly / SRE Weekly

**Subject:** Zero-ops LLM infrastructure: one binary, no database to manage

Stockyard replaces the typical LLM infrastructure stack (proxy + observability + audit + cost tracking) with a single Go binary. No Redis, no Postgres, no Docker Compose. Embedded SQLite with WAL mode. Starts in 50ms. Self-hosted, MIT licensed.

Relevant for teams running LLM-powered apps who want observability and cost controls without adding infrastructure.

https://stockyard.dev
https://github.com/stockyard-dev/Stockyard
