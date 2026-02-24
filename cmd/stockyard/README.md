# Stockyard Suite

**Every LLM tool you need. One binary.**

The full Stockyard suite — all 125 products in a single binary. Cost caps, caching, rate limiting, failover, logging, and 120 more middleware tools.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/stockyard

# Your app:   http://localhost:4000/v1/chat/completions
# Dashboard:  http://localhost:4000/ui
```

## What You Get

- All 125 products in one binary
- Single YAML config for everything
- Unified dashboard
- 63-step middleware chain
- 17+ provider adapters
- 6MB static binary, zero dependencies

## Config

```yaml
# stockyard.yaml
port: 4000
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}

# Enable products by adding their config sections:
costcap:
  enabled: true
  projects:
    default:
      caps: { daily: 10.00 }

cache:
  enabled: true
  ttl: 3600

rateshield:
  enabled: true
  default:
    requests_per_minute: 60
```

## Docker

```bash
docker run -p 4000:4000 -e OPENAI_API_KEY=sk-... stockyard/stockyard
```

## Part of Stockyard

Stockyard Suite is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use Stockyard Suite standalone.
