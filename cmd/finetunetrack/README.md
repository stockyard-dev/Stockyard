# FineTuneTrack

**Monitor fine-tuned model performance.**

FineTuneTrack runs evaluation suites against your fine-tuned models periodically. Track scores and alert on degradation.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/finetunetrack

# Your app:   http://localhost:6520/v1/chat/completions
# Dashboard:  http://localhost:6520/ui
```

## What You Get

- Periodic evaluation runs
- Score tracking over time
- Base model comparison
- Degradation alerts
- Data distribution monitoring
- Dashboard with performance trends

## Config

```yaml
# finetunetrack.yaml
port: 6520
finetunetrack:
  models:
    - id: ft:gpt-4o-mini:my-org:custom:id
      eval_suite: ./evals/
      schedule: "0 0 * * 0"  # weekly
      baseline: gpt-4o-mini
```

## Docker

```bash
docker run -p 6520:6520 -e OPENAI_API_KEY=sk-... stockyard/finetunetrack
```

## Part of Stockyard

FineTuneTrack is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use FineTuneTrack standalone.
