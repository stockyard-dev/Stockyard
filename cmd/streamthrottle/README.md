# StreamThrottle

**Control streaming speed.**

StreamThrottle limits tokens per second in streaming responses. Buffer fast streams for better UX or reading speed.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/streamthrottle

# Your app:   http://localhost:6390/v1/chat/completions
# Dashboard:  http://localhost:6390/ui
```

## What You Get

- Max tokens per second
- Buffer fast streams
- Configurable per endpoint
- Model-specific speeds
- Client-specific throttling
- Dashboard with speed metrics

## Config

```yaml
# streamthrottle.yaml
port: 6390
streamthrottle:
  max_tokens_per_sec: 30
  buffer: true
```

## Docker

```bash
docker run -p 6390:6390 -e OPENAI_API_KEY=sk-... stockyard/streamthrottle
```

## Part of Stockyard

StreamThrottle is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use StreamThrottle standalone.
