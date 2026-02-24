# LangBridge

**Multilingual LLM middleware.**

LangBridge detects input language, translates to English for the model, and translates the response back. Cached translations reduce cost.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/langbridge

# Your app:   http://localhost:5900/v1/chat/completions
# Dashboard:  http://localhost:5900/ui
```

## What You Get

- Automatic language detection
- Input translation to English
- Response translation back to user language
- Translation caching
- Language pair cost tracking
- Configurable source/target languages

## Config

```yaml
# langbridge.yaml
port: 5900
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
langbridge:
  enabled: true
  model_language: en
  cache_translations: true
  supported_languages: [es, fr, de, ja, ko, zh, pt, it]
```

## Docker

```bash
docker run -p 5900:5900 -e OPENAI_API_KEY=sk-... stockyard/langbridge
```

## Part of Stockyard

LangBridge is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use LangBridge standalone.
