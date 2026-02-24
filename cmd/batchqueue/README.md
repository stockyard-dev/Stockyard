# BatchQueue

**Queue it up. Process it later.**

BatchQueue provides async request queuing with configurable concurrency control. Submit jobs, get results via polling or webhook.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/batchqueue

# Your app:   http://localhost:5000/v1/chat/completions
# Dashboard:  http://localhost:5000/ui
```

## What You Get

- Async request queue
- Configurable concurrency limits
- Job status polling API
- Webhook on completion
- Priority levels
- Dashboard with queue depth

## Config

```yaml
# batchqueue.yaml
port: 5000
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
queue:
  max_concurrent: 5
  max_queue_depth: 1000
  timeout: 120s
  webhook_on_complete: ""
```

## Docker

```bash
docker run -p 5000:5000 -e OPENAI_API_KEY=sk-... stockyard/batchqueue
```

## Part of Stockyard

BatchQueue is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use BatchQueue standalone.
