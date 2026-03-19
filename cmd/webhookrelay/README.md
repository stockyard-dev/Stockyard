# WebhookRelay

**Trigger LLM calls from any webhook.**

WebhookRelay exposes inbound webhook endpoints that extract data, build prompts, call LLMs, and send results to destinations.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/webhookrelay

# Your app:   http://localhost:5950/v1/chat/completions
# Dashboard:  http://localhost:5950/ui
```

## What You Get

- Inbound webhook endpoints
- Configurable data extraction
- Template-based prompt building
- Result forwarding to webhooks
- GitHub/Slack/Discord triggers
- Dashboard with trigger history

## Config

```yaml
# webhookrelay.yaml
port: 5950
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
webhooks:
  github_summarize:
    trigger: /webhook/github
    extract: "body.issue.body"
    prompt: "Summarize this GitHub issue: {{extracted}}"
    model: gpt-4o-mini
    forward: ${SLACK_WEBHOOK}
```

## Docker

```bash
docker run -p 5950:5950 -e OPENAI_API_KEY=sk-... stockyard/webhookrelay
```

## Part of Stockyard

WebhookRelay is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
