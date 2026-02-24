# RegionRoute

**Data residency routing for GDPR.**

RegionRoute routes requests to region-specific provider endpoints based on tenant, header, or IP geolocation. Keep EU data in EU.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/regionroute

# Your app:   http://localhost:5920/v1/chat/completions
# Dashboard:  http://localhost:5920/ui
```

## What You Get

- Region-based request routing
- Header/tenant/IP geolocation routing
- Map regions to endpoints
- ComplianceLog integration
- GDPR data residency compliance
- Dashboard with regional traffic

## Config

```yaml
# regionroute.yaml
port: 5920
providers:
  openai_us:
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
  openai_eu:
    base_url: https://eu.api.openai.com/v1
    api_key: ${OPENAI_API_KEY_EU}
regionroute:
  rules:
    - region: EU
      provider: openai_eu
    - region: "*"
      provider: openai_us
  detect_by: header  # header | ip | tenant
```

## Docker

```bash
docker run -p 5920:5920 -e OPENAI_API_KEY=sk-... stockyard/regionroute
```

## Part of Stockyard

RegionRoute is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use RegionRoute standalone.
