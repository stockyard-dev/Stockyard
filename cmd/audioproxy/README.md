# AudioProxy

**Proxy for speech-to-text and TTS APIs.**

AudioProxy provides caching, cost tracking, and failover for Whisper STT and ElevenLabs/OpenAI TTS calls.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/audioproxy

# Your app:   http://localhost:6160/v1/chat/completions
# Dashboard:  http://localhost:6160/ui
```

## What You Get

- TTS response caching (text hash to audio)
- Per-minute STT cost tracking
- Provider failover for audio
- Format conversion
- Latency optimization
- Dashboard with audio metrics

## Config

```yaml
# audioproxy.yaml
port: 6160
audioproxy:
  tts_cache: true
  stt_providers:
    - whisper
  tts_providers:
    - openai
    - elevenlabs
```

## Docker

```bash
docker run -p 6160:6160 -e OPENAI_API_KEY=sk-... stockyard/audioproxy
```

## Part of Stockyard

AudioProxy is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AudioProxy standalone.
