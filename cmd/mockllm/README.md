# MockLLM

**Deterministic LLM responses for testing.**

MockLLM provides a fake LLM server with fixture-based responses. Perfect for CI/CD — no API keys, no costs, deterministic behavior.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/mockllm

# Your app:   http://localhost:5660/v1/chat/completions
# Dashboard:  http://localhost:5660/ui
```

## What You Get

- Fixture-based responses
- Prompt pattern matching
- Regex and exact match modes
- Configurable latency simulation
- Error simulation
- Zero API costs in CI

## Config

```yaml
# mockllm.yaml
port: 5660
providers: {}  # No real providers needed
mock:
  fixtures:
    - match: "hello"
      response: "Hi! How can I help you?"
    - match: ".*json.*"
      type: regex
      response: '{"answer": "mock response"}'
  default_response: "This is a mock response."
  latency_ms: 100  # Simulate real latency
```

## Docker

```bash
docker run -p 5660:5660 -e OPENAI_API_KEY=sk-... stockyard/mockllm
```

## Part of Stockyard

MockLLM is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use MockLLM standalone.
