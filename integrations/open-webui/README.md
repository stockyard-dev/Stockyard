# Stockyard × Open WebUI

> Cost caps, caching, rate limiting, and analytics for Open WebUI — zero config.

Open WebUI (124K+ ★) is the most popular self-hosted AI interface. Stockyard sits between Open WebUI and your LLM providers, adding cost control, caching, PII redaction, failover routing, and real-time analytics.

## 3 Ways to Integrate

### Option 1: Docker Sidecar (Recommended)

The fastest path — run Open WebUI and Stockyard together:

```bash
OPENAI_API_KEY=sk-... docker compose up
```

Open WebUI auto-connects to Stockyard. Dashboard at `http://localhost:4000/ui`.

### Option 2: Pipeline Plugin

For existing Open WebUI installations. Install `stockyard_pipeline.py` in Admin → Pipelines:

1. Start Stockyard: `npx @stockyard/stockyard`
2. In Open WebUI: Admin → Pipelines → Upload → select `stockyard_pipeline.py`
3. Configure the proxy URL in pipeline settings

This routes ALL LLM requests through Stockyard with full streaming support.

### Option 3: Cost Filter

Lightweight — just adds spend tracking to responses without rerouting:

1. Upload `stockyard_filter.py` as a filter in Admin → Pipelines
2. Every response shows running cost, cache hit rate, and budget alerts

## What You Get

| Feature | What it does |
|---------|-------------|
| **CostCap** | Daily/monthly spending limits — auto-blocks when budget is hit |
| **CacheLayer** | Same prompt = instant cached response, saves 30-50% |
| **RateShield** | Prevents 429 errors from hammering your API keys |
| **PromptGuard** | Strips PII before it reaches the LLM provider |
| **FallbackRouter** | OpenAI down? Auto-routes to Anthropic/Groq |
| **LLMTap** | Full analytics — latency, costs, errors, all in one dashboard |

## Configuration

Set environment variables or edit Stockyard's YAML config:

```yaml
# stockyard.yaml
port: 4000
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
projects:
  default:
    provider: openai
    model: gpt-4o-mini
    caps:
      daily: 10.00
      monthly: 100.00
cache:
  ttl: 1h
```

## Files

| File | Description |
|------|-------------|
| `docker-compose.yml` | One-command Open WebUI + Stockyard sidecar |
| `stockyard_pipeline.py` | Full pipeline — reroutes all requests through Stockyard |
| `stockyard_filter.py` | Lightweight filter — adds cost/cache info to responses |

## License

MIT
