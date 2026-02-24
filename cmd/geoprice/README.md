# GeoPrice

**Purchasing power pricing by region.**

GeoPrice adjusts pricing based on user region using purchasing power parity. Anti-VPN detection included.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/geoprice

# Your app:   http://localhost:6600/v1/chat/completions
# Dashboard:  http://localhost:6600/ui
```

## What You Get

- PPP-adjusted pricing
- Region detection
- Anti-VPN checks
- Revenue by region tracking
- Configurable multipliers
- Dashboard with regional revenue

## Config

```yaml
# geoprice.yaml
port: 6600
geoprice:
  multipliers:
    US: 1.0
    EU: 0.9
    BR: 0.4
    IN: 0.3
  detect_vpn: true
```

## Docker

```bash
docker run -p 6600:6600 -e OPENAI_API_KEY=sk-... stockyard/geoprice
```

## Part of Stockyard

GeoPrice is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use GeoPrice standalone.
