# FeedbackLoop

**Collect user feedback. Close the loop.**

FeedbackLoop captures user ratings linked to specific LLM requests. Track which prompts produce bad responses and export for improvement.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/feedbackloop

# Your app:   http://localhost:5770/v1/chat/completions
# Dashboard:  http://localhost:5770/ui
```

## What You Get

- Per-request feedback capture
- Thumbs up/down and ratings
- Link feedback to request IDs
- Worst-performing prompt reports
- Export for fine-tuning
- Dashboard with feedback trends

## Config

```yaml
# feedbackloop.yaml
port: 5770
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
feedback:
  enabled: true
  endpoint: /api/feedback  # POST {request_id, rating, comment}
  retention_days: 90
```

## Docker

```bash
docker run -p 5770:5770 -e OPENAI_API_KEY=sk-... stockyard/feedbackloop
```

## Part of Stockyard

FeedbackLoop is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
