# AlertPulse

**PagerDuty for your LLM stack.**

AlertPulse monitors error rates, latency, and costs with configurable alert rules. Fire webhooks to Slack, Discord, PagerDuty, or email.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/alertpulse

# Your app:   http://localhost:5640/v1/chat/completions
# Dashboard:  http://localhost:5640/ui
```

## What You Get

- Configurable alert rules
- Error rate, latency, cost thresholds
- Slack/Discord/PagerDuty webhooks
- Cooldown periods
- Sliding window metrics
- Dashboard with alert history

## Config

```yaml
# alertpulse.yaml
port: 5640
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
alerts:
  rules:
    - name: high_errors
      metric: error_rate
      threshold: 25
      webhook: ${ALERT_WEBHOOK}
    - name: cost_spike
      metric: cost_per_min
      threshold: 1.0
      webhook: ${ALERT_WEBHOOK}
  cooldown: 5m
```

## Docker

```bash
docker run -p 5640:5640 -e OPENAI_API_KEY=sk-... stockyard/alertpulse
```

## Part of Stockyard

AlertPulse is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AlertPulse standalone.
