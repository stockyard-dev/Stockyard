# PlaybackStudio

**Interactive playground for logged requests.**

PlaybackStudio provides rich exploration of logged interactions. Advanced filters, conversation threads, side-by-side comparison, and content search.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/playbackstudio

# Your app:   http://localhost:6630/v1/chat/completions
# Dashboard:  http://localhost:6630/ui
```

## What You Get

- Advanced request filtering
- Conversation thread view
- Side-by-side comparison
- Content search
- Bulk actions
- Interactive dashboard

## Config

```yaml
# playbackstudio.yaml
port: 6630
playbackstudio:
  source: promptreplay
  search_index: true
  max_results: 1000
```

## Docker

```bash
docker run -p 6630:6630 -e OPENAI_API_KEY=sk-... stockyard/playbackstudio
```

## Part of Stockyard

PlaybackStudio is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PlaybackStudio standalone.
