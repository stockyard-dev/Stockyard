# CliDash

**Terminal dashboard for your LLM stack.**

CliDash provides an htop-style terminal UI for monitoring Stockyard. Real-time req/sec, cache stats, spend, and errors.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/clidash

# Your app:   http://localhost:6500/v1/chat/completions
# Dashboard:  http://localhost:6500/ui
```

## What You Get

- Terminal-native monitoring (TUI)
- Real-time metrics display
- Keyboard drill-down
- SSH-accessible
- No browser needed
- bubbletea-based rendering

## Config

```yaml
# clidash.yaml
port: 6500
clidash:
  refresh_interval: 1s
  panels: [requests, cache, spend, errors, models]
```

## Docker

```bash
docker run -p 6500:6500 -e OPENAI_API_KEY=sk-... stockyard/clidash
```

## Part of Stockyard

CliDash is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
