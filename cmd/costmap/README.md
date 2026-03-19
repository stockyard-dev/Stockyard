# CostMap

**Multi-dimensional cost attribution.**

CostMap tags requests with dimensions and provides interactive drill-down cost analytics.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/costmap

# Your app:   http://localhost:6290/v1/chat/completions
# Dashboard:  http://localhost:6290/ui
```

## What You Get

- Tag-based cost attribution
- Multi-dimensional drill-down
- Interactive dashboard
- Export to BI tools
- Per-feature cost tracking
- Budget vs actual per dimension

## Config

```yaml
# costmap.yaml
port: 6290
costmap:
  dimensions:
    - header: x-feature
    - header: x-team
    - model
```

## Docker

```bash
docker run -p 6290:6290 -e OPENAI_API_KEY=sk-... stockyard/costmap
```

## Part of Stockyard

CostMap is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
