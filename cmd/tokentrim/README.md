# TokenTrim

**Fit more into your context window.**

TokenTrim optimizes context window usage with smart truncation strategies. Prioritize recent messages, trim system prompts, or use custom strategies.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tokentrim

# Your app:   http://localhost:4900/v1/chat/completions
# Dashboard:  http://localhost:4900/ui
```

## What You Get

- Smart context window truncation
- Prioritize recent messages
- System prompt compression
- Configurable strategies per model
- Token count visibility
- Works with any context window size

## Config

```yaml
# tokentrim.yaml
port: 4900
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
truncation:
  strategy: recent_first  # recent_first | oldest_first | smart
  reserve_system: 500     # tokens reserved for system prompt
  reserve_response: 1000  # tokens reserved for response
  target_ratio: 0.8       # fill to 80% of context window
```

## Docker

```bash
docker run -p 4900:4900 -e OPENAI_API_KEY=sk-... stockyard/tokentrim
```

## Part of Stockyard

TokenTrim is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TokenTrim standalone.
