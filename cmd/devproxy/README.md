# DevProxy

**Charles Proxy for LLM APIs.**

DevProxy provides an interactive debugging dashboard for LLM traffic. Inspect, pause, edit, and replay requests in real-time.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/devproxy

# Your app:   http://localhost:5820/v1/chat/completions
# Dashboard:  http://localhost:5820/ui
```

## What You Get

- Live WebSocket request inspector
- Pause/resume request flow
- Edit requests before forwarding
- Breakpoints on patterns
- Request/response diff view
- Interactive debugging dashboard

## Config

```yaml
# devproxy.yaml
port: 5820
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
devproxy:
  capture: true
  breakpoints:
    - pattern: "DELETE"
    - header: "x-debug: true"
  websocket_port: 5821
```

## Docker

```bash
docker run -p 5820:5820 -e OPENAI_API_KEY=sk-... stockyard/devproxy
```

## Part of Stockyard

DevProxy is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use DevProxy standalone.
