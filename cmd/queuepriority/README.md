# QueuePriority

**Priority queues for LLM requests.**

QueuePriority extends BatchQueue with priority levels. Enterprise customers jump ahead of free tier. Reserved capacity and SLA tracking.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/queuepriority

# Your app:   http://localhost:6590/v1/chat/completions
# Dashboard:  http://localhost:6590/ui
```

## What You Get

- Priority levels per key/tenant
- Reserved capacity
- SLA tracking
- Queue depth per priority
- Auto-promote on timeout
- Dashboard with queue analytics

## Config

```yaml
# queuepriority.yaml
port: 6590
queuepriority:
  levels:
    critical: { weight: 10, reserved: 5 }
    high: { weight: 5 }
    normal: { weight: 1 }
    low: { weight: 0 }
```

## Docker

```bash
docker run -p 6590:6590 -e OPENAI_API_KEY=sk-... stockyard/queuepriority
```

## Part of Stockyard

QueuePriority is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use QueuePriority standalone.
