# AnthroFit

**Use Claude with OpenAI SDKs.**

AnthroFit provides deep translation between OpenAI and Anthropic API formats. System messages, tool schemas, streaming format, and response structure all handled.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/anthrofit

# Your app:   http://localhost:5710/v1/chat/completions
# Dashboard:  http://localhost:5710/ui
```

## What You Get

- OpenAI-to-Anthropic API translation
- System message conversion
- Tool/function call schema mapping
- Streaming format translation
- Response structure normalization
- Drop-in Anthropic support

## Config

```yaml
# anthrofit.yaml
port: 5710
providers:
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
anthrofit:
  enabled: true
  default_model: claude-sonnet-4-20250514
  model_map:
    gpt-4o: claude-sonnet-4-20250514
    gpt-4o-mini: claude-haiku-4-5-20251001
```

## Docker

```bash
docker run -p 5710:5710 -e OPENAI_API_KEY=sk-... stockyard/anthrofit
```

## Part of Stockyard

AnthroFit is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AnthroFit standalone.
