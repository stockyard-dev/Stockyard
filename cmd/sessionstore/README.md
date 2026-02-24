# SessionStore

**Managed conversation sessions.**

SessionStore provides CRUD operations for conversation sessions. Create, resume, list, delete, share, and export sessions.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/sessionstore

# Your app:   http://localhost:6190/v1/chat/completions
# Dashboard:  http://localhost:6190/ui
```

## What You Get

- Session CRUD API
- Full history persistence
- Metadata per session
- Concurrent session limits
- Session sharing
- Export as training data

## Config

```yaml
# sessionstore.yaml
port: 6190
sessionstore:
  max_sessions_per_user: 100
  max_history: 1000
  auto_expire: 30d
```

## Docker

```bash
docker run -p 6190:6190 -e OPENAI_API_KEY=sk-... stockyard/sessionstore
```

## Part of Stockyard

SessionStore is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SessionStore standalone.
