# CostCap

**Never get a surprise LLM bill again.**

CostCap is an OpenAI-compatible proxy that enforces spending caps on your LLM API calls. Set a daily or monthly budget, get alerts at configurable thresholds, and never wake up to a $500 bill because a loop went crazy overnight.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/costcap

# Your app → http://localhost:4100/v1/chat/completions
# Dashboard → http://localhost:4100/ui
```

## What You Get

- **Hard spending caps** — requests return 429 when you hit your limit
- **Per-project budgets** — different caps for dev, staging, prod
- **Alert webhooks** — Slack/Discord notifications at 50%, 80%, 95%
- **Live dashboard** — watch your spend tick up in real-time
- **Per-model cost breakdown** — see which models are eating your budget
- **Zero code changes** — just change your base URL

## Config

```yaml
# costcap.yaml
port: 4100
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
projects:
  default:
    caps:
      daily: 5.00
      monthly: 50.00
    alerts:
      webhook: https://hooks.slack.com/...
      thresholds: [50, 80, 95]
```

Part of [Stockyard](https://github.com/stockyard-dev/stockyard) — every LLM tool you need, one binary.
