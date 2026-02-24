# VisionProxy

**Proxy for vision/image-understanding APIs.**

VisionProxy handles GPT-4V and Claude vision requests with image-specific caching, cost tracking, resizing, and failover.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/visionproxy

# Your app:   http://localhost:6150/v1/chat/completions
# Dashboard:  http://localhost:6150/ui
```

## What You Get

- Image input detection and hashing
- Vision-specific caching
- Per-image cost tracking
- Auto-resize and compress
- Provider failover for vision
- Dashboard with vision metrics

## Config

```yaml
# visionproxy.yaml
port: 6150
visionproxy:
  cache: true
  resize_max: 2048
  compress_quality: 85
  cost_per_image: { gpt-4o: 0.01 }
```

## Docker

```bash
docker run -p 6150:6150 -e OPENAI_API_KEY=sk-... stockyard/visionproxy
```

## Part of Stockyard

VisionProxy is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use VisionProxy standalone.
