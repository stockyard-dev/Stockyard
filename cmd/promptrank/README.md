# PromptRank

**Rank prompts by ROI.**

PromptRank combines cost, quality score, latency, volume, and feedback into a per-template ROI index. Find your best and worst prompts.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptrank

# Your app:   http://localhost:6460/v1/chat/completions
# Dashboard:  http://localhost:6460/ui
```

## What You Get

- Per-template ROI index
- Cost/quality/latency ranking
- Volume-weighted scoring
- Feedback integration
- Prompt leaderboard
- Dashboard with ROI charts

## Config

```yaml
# promptrank.yaml
port: 6460
promptrank:
  metrics: [cost, quality, latency, volume, feedback]
  weight: { quality: 0.4, cost: 0.3, latency: 0.2, volume: 0.1 }
```

## Docker

```bash
docker run -p 6460:6460 -e OPENAI_API_KEY=sk-... stockyard/promptrank
```

## Part of Stockyard

PromptRank is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PromptRank standalone.
