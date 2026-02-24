# ChatMem

**Persistent memory without eating your context window.**

ChatMem manages conversation memory with smart strategies. Sliding window, summarization, and importance-based retention keep context relevant without burning tokens.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/chatmem

# Your app:   http://localhost:5650/v1/chat/completions
# Dashboard:  http://localhost:5650/ui
```

## What You Get

- Session-based conversation memory
- Sliding window strategy
- Auto-summarization of old messages
- Importance-based retention
- Configurable memory budget
- Dashboard with session explorer

## Config

```yaml
# chatmem.yaml
port: 5650
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
memory:
  strategy: sliding_window  # sliding_window | summarize | importance
  max_tokens: 4000
  summarize_after: 20  # messages
  session_header: X-Session-ID
```

## Docker

```bash
docker run -p 5650:5650 -e OPENAI_API_KEY=sk-... stockyard/chatmem
```

## Part of Stockyard

ChatMem is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ChatMem standalone.
