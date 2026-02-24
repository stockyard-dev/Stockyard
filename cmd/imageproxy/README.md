# ImageProxy

**Proxy magic for image generation APIs.**

ImageProxy extends the proxy to /v1/images/generations. Cost tracking per image, prompt-hash caching, and provider failover for DALL-E, Stable Diffusion, etc.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/imageproxy

# Your app:   http://localhost:5890/v1/chat/completions
# Dashboard:  http://localhost:5890/ui
```

## What You Get

- Image generation API proxy
- Per-image cost tracking
- Prompt hash caching
- Provider failover
- Size and quality controls
- Dashboard with generation history

## Config

```yaml
# imageproxy.yaml
port: 5890
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
imageproxy:
  cache: true
  default_size: 1024x1024
  max_per_day: 100
  cost_per_image:
    dall-e-3: 0.04
    dall-e-2: 0.02
```

## Docker

```bash
docker run -p 5890:5890 -e OPENAI_API_KEY=sk-... stockyard/imageproxy
```

## Part of Stockyard

ImageProxy is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ImageProxy standalone.
