# GeminiShim

**Tame Gemini's quirks.**

GeminiShim handles Gemini-specific issues: random safety filter blocks, inconsistent JSON mode, and multimodal format differences.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/geminishim

# Your app:   http://localhost:5800/v1/chat/completions
# Dashboard:  http://localhost:5800/ui
```

## What You Get

- Auto-retry on safety filter blocks
- JSON mode normalization
- Multimodal format translation
- Token count normalization
- Gemini-specific error handling
- Drop-in Gemini support

## Config

```yaml
# geminishim.yaml
port: 5800
providers:
  gemini:
    api_key: ${GEMINI_API_KEY}
geminishim:
  retry_safety_blocks: true
  max_safety_retries: 3
  normalize_json: true
  model_map:
    gpt-4o: gemini-1.5-pro
    gpt-4o-mini: gemini-1.5-flash
```

## Docker

```bash
docker run -p 5800:5800 -e OPENAI_API_KEY=sk-... stockyard/geminishim
```

## Part of Stockyard

GeminiShim is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use GeminiShim standalone.
