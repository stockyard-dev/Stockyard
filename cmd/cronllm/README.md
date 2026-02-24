# CronLLM

**Scheduled LLM tasks.**

CronLLM runs LLM prompts on cron schedules. Daily summaries, weekly reports, periodic content generation — all through the proxy chain.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/cronllm

# Your app:   http://localhost:5940/v1/chat/completions
# Dashboard:  http://localhost:5940/ui
```

## What You Get

- Cron-scheduled LLM calls
- YAML job definitions
- Output to file, webhook, or email
- Full proxy chain per job
- Job history and logs
- Dashboard with schedule overview

## Config

```yaml
# cronllm.yaml
port: 5940
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
cron:
  jobs:
    - name: daily_summary
      schedule: "0 9 * * *"
      model: gpt-4o-mini
      prompt: "Summarize yesterday's key events in tech."
      output: webhook
      webhook_url: ${SLACK_WEBHOOK}
```

## Docker

```bash
docker run -p 5940:5940 -e OPENAI_API_KEY=sk-... stockyard/cronllm
```

## Part of Stockyard

CronLLM is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use CronLLM standalone.
