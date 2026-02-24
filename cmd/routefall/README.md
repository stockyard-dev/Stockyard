# FallbackRouter

**Your app never goes down because OpenAI did.**

FallbackRouter automatically routes to a backup provider when your primary is down. Circuit breakers, health checks, and automatic recovery — your users never notice.

## Quickstart

```bash
export OPENAI_API_KEY=sk-... ANTHROPIC_API_KEY=sk-ant-...
npx @stockyard/routefall

# Dashboard → http://localhost:4103/ui
```

## What You Get

- **Automatic failover** across OpenAI, Anthropic, Groq, Gemini
- **Circuit breakers** — stop hammering a dead provider
- **Provider health dashboard** — latency, status, uptime
- **Failover timeline** — see every switch with timestamps
- **99.99% effective uptime** — your app stays up

Part of [Stockyard](https://github.com/stockyard-dev/stockyard).
