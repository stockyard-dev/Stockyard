# MirrorTest

**Shadow test new models against production.**

MirrorTest sends production traffic to a shadow model for comparison. Primary response goes to the user; shadow is logged for analysis.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/mirrortest

# Your app:   http://localhost:6070/v1/chat/completions
# Dashboard:  http://localhost:6070/ui
```

## What You Get

- Shadow model testing
- Zero user impact
- Quality comparison logging
- Latency and cost comparison
- Configurable shadow percentage
- Dashboard with comparison results

## Config

```yaml
# mirrortest.yaml
port: 6070
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
mirror:
  primary: gpt-4o
  shadow: gpt-4o-mini
  shadow_percent: 10  # test 10% of traffic
  log_comparison: true
```

## Docker

```bash
docker run -p 6070:6070 -e OPENAI_API_KEY=sk-... stockyard/mirrortest
```

## Part of Stockyard

MirrorTest is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use MirrorTest standalone.
