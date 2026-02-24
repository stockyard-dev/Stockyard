# ToolMock

**Fake tool responses for testing.**

ToolMock intercepts tool_result messages and returns canned responses. Test tool-use agents without real external services.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/toolmock

# Your app:   http://localhost:6120/v1/chat/completions
# Dashboard:  http://localhost:6120/ui
```

## What You Get

- Canned tool responses
- Match by tool name and arguments
- Simulate errors and timeouts
- Partial result simulation
- Deterministic for CI
- Zero external dependencies

## Config

```yaml
# toolmock.yaml
port: 6120
toolmock:
  mocks:
    get_weather:
      response: { temp: 72, condition: sunny }
    search:
      response: { results: [{ title: "Mock result" }] }
```

## Docker

```bash
docker run -p 6120:6120 -e OPENAI_API_KEY=sk-... stockyard/toolmock
```

## Part of Stockyard

ToolMock is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ToolMock standalone.
