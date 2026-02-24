# Why I Built Stockyard

I was building an app with the OpenAI API. Everything worked great in development. Then I shipped it.

Within the first week, three things happened:

1. My API bill hit $400 and I didn't notice until the invoice.
2. OpenAI went down for 45 minutes on a Tuesday afternoon. My app went down with it.
3. I realized I was paying for the exact same API response dozens of times because users kept asking similar questions.

These are not hard problems. Cost caps, failover routing, and response caching are solved problems in every other part of the stack. But for LLM APIs? You're on your own.

I looked at the options. LiteLLM is the big one — 35,000 GitHub stars. But it's Python, it needs Redis and Postgres, and it's one monolithic proxy that tries to do everything. I just wanted a spending cap. I didn't want to set up an entire infrastructure stack for it.

So I built what I wanted: a single binary I could download and run in 10 seconds.

## What Stockyard is

Stockyard is a Go proxy that sits between your app and any LLM provider. You don't change your application code — you just point your `OPENAI_BASE_URL` at Stockyard instead of directly at OpenAI.

```bash
# Install
brew install stockyard-dev/tap/stockyard

# Run
export OPENAI_API_KEY=sk-...
stockyard
```

That's it. Your app now has cost tracking, response caching, rate limiting, and provider failover. There's a dashboard at `localhost:4000/ui` where you can see everything happening in real-time.

## Why Go, why single binary

I'm tired of tools that need Docker Compose files with 4 services, a Postgres database, and a Redis instance just to add a spending cap to my API calls.

Stockyard is a single static binary. No runtime dependencies. No database to manage (it uses embedded SQLite). No Docker required (though we have images if you want them). Download, run, done.

It starts in under 50ms, uses about 12MB of memory, and cross-compiles to every platform. That's the Go advantage.

## 20 products, one binary

The proxy is built as composable middleware. Each "product" is a middleware in the chain. You can run them individually or all together:

- **CostCap** — Spend tracking with hard/soft caps
- **CacheLayer** — Response caching (exact and semantic matching)
- **FallbackRouter** — Provider failover with circuit breakers
- **RateShield** — Rate limiting with per-key limits
- **PromptGuard** — PII redaction and injection detection
- **KeyPool** — API key rotation
- **LLMTap** — Analytics dashboard (p50/p95/p99 latency, cost trends)
- And 13 more

Each one has its own embedded dashboard and works with OpenAI, Anthropic, Gemini, Groq, Ollama, and 12 more providers.

## Pricing

One product free forever (1,000 requests/day). Individual products at $9.99/mo. All 20 for $29.99/mo.

No credit card for the free tier. Download and run.

## Try it

```bash
brew install stockyard-dev/tap/stockyard
# or: npx @stockyard/stockyard
# or: docker run -p 4000:4000 ghcr.io/stockyard-dev/stockyard
```

- Website: [stockyard.dev](https://stockyard.dev)
- GitHub: [github.com/stockyard-dev/stockyard](https://github.com/stockyard-dev/stockyard)
- Docs: [stockyard.dev/docs](https://stockyard.dev/docs)

I'd love to hear what LLM infrastructure problems you're running into. What would you want a proxy like this to do?

---

*Stockyard. Where LLM traffic gets sorted.*
