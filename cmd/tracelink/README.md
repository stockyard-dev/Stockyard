# TraceLink

**Distributed tracing for LLM chains.**

TraceLink propagates trace IDs across multi-step LLM calls. Link parent-child requests into trees for debugging agent workflows.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tracelink

# Your app:   http://localhost:5630/v1/chat/completions
# Dashboard:  http://localhost:5630/ui
```

## What You Get

- X-Trace-ID propagation
- Parent-child request linking
- Waterfall visualization
- OpenTelemetry compatible
- Per-trace cost and latency
- Dashboard with trace explorer

## Config

```yaml
# tracelink.yaml
port: 5630
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
tracing:
  enabled: true
  header: X-Trace-ID
  auto_generate: true
  otlp_export: ""  # optional OTLP endpoint
```

## Docker

```bash
docker run -p 5630:5630 -e OPENAI_API_KEY=sk-... stockyard/tracelink
```

## Part of Stockyard

TraceLink is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TraceLink standalone.
