# VoiceBridge

**LLM output optimized for voice.**

VoiceBridge strips markdown, converts lists to prose, removes code blocks, and enforces max length for TTS-friendly output.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/voicebridge

# Your app:   http://localhost:5880/v1/chat/completions
# Dashboard:  http://localhost:5880/ui
```

## What You Get

- Strip markdown from output
- Convert lists to natural prose
- Remove code blocks
- Max length enforcement
- TTFB tracking for voice latency
- Configurable output style

## Config

```yaml
# voicebridge.yaml
port: 5880
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
voicebridge:
  strip_markdown: true
  convert_lists: true
  remove_code: true
  max_length: 300  # characters
  style: conversational
```

## Docker

```bash
docker run -p 5880:5880 -e OPENAI_API_KEY=sk-... stockyard/voicebridge
```

## Part of Stockyard

VoiceBridge is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use VoiceBridge standalone.
