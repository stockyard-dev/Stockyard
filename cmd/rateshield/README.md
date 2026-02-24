# RateShield

**Protect your API keys from abuse and runaway loops.**

RateShield adds rate limiting to your LLM proxy. Per-user, per-IP, configurable burst — stop a single bad actor (or a broken loop) from burning through your budget.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/rateshield

# Dashboard → http://localhost:4104/ui
```

## What You Get

- **Per-user and per-IP rate limits**
- **Configurable burst** — allow spikes, block sustained abuse
- **Live request rate chart** — see traffic in real-time
- **Top rate-limited IPs** — identify abusers
- **429 responses** with retry-after headers

Part of [Stockyard](https://github.com/stockyard-dev/stockyard).
