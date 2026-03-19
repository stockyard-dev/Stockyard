# OutputCap

**Stop paying for responses you don't need.**

OutputCap monitors token count in streaming responses and cuts at natural sentence boundaries. Ask for one word, don't pay for an essay.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/outputcap

# Your app:   http://localhost:5860/v1/chat/completions
# Dashboard:  http://localhost:5860/ui
```

## What You Get

- Natural boundary detection
- Sentence-aware truncation
- Token budget per request
- Streaming-aware cutting
- Cost savings tracking
- Configurable per model

## Config

```yaml
# outputcap.yaml
port: 5860
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
outputcap:
  max_tokens: 500
  cut_at: sentence  # sentence | paragraph | word
  warn_header: true  # Add X-Output-Capped header
```

## Docker

```bash
docker run -p 5860:5860 -e OPENAI_API_KEY=sk-... stockyard/outputcap
```

## Part of Stockyard

OutputCap is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
