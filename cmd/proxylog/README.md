# ProxyLog

**Structured logging for every proxy decision.**

ProxyLog instruments each middleware to emit decision logs. See WHY provider B was chosen over A, WHY a cache miss happened.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/proxylog

# Your app:   http://localhost:6490/v1/chat/completions
# Dashboard:  http://localhost:6490/ui
```

## What You Get

- Per-middleware decision logging
- X-Proxy-Trace header
- Full request decision trace
- Searchable decision history
- Middleware timing breakdown
- Dashboard with decision explorer

## Config

```yaml
# proxylog.yaml
port: 6490
proxylog:
  enabled: true
  log_decisions: true
  trace_header: X-Proxy-Trace
  retention_days: 30
```

## Docker

```bash
docker run -p 6490:6490 -e OPENAI_API_KEY=sk-... stockyard/proxylog
```

## Part of Stockyard

ProxyLog is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ProxyLog standalone.
