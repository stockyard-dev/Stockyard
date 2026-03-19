# FrameGrab

**Video frame extraction for vision LLMs.**

FrameGrab extracts frames from video, batches them through vision LLMs, and caches analyses. Smart frame selection by scene change.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/framegrab

# Your app:   http://localhost:6180/v1/chat/completions
# Dashboard:  http://localhost:6180/ui
```

## What You Get

- Scene-based frame extraction
- Batch frame analysis
- Per-frame caching
- Smart frame selection
- Cost per frame tracking
- Pipeline API

## Config

```yaml
# framegrab.yaml
port: 6180
framegrab:
  extract_mode: scene_change
  max_frames: 20
  cache: true
  vision_model: gpt-4o
```

## Docker

```bash
docker run -p 6180:6180 -e OPENAI_API_KEY=sk-... stockyard/framegrab
```

## Part of Stockyard

FrameGrab is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
