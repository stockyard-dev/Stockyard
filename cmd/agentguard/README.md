# AgentGuard

**Safety rails for autonomous agents.**

AgentGuard tracks agent sessions and enforces per-session limits on calls, cost, duration, and allowed tools.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/agentguard

# Your app:   http://localhost:5720/v1/chat/completions
# Dashboard:  http://localhost:5720/ui
```

## What You Get

- Per-session call limits
- Per-session cost caps
- Max session duration
- Allowed tool restrictions
- Kill session on breach
- Dashboard with session monitor

## Config

```yaml
# agentguard.yaml
port: 5720
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
agentguard:
  session_header: X-Session-ID
  max_calls: 50
  max_cost: 5.00
  max_duration: 300s
  allowed_tools: []  # empty = all allowed
```

## Docker

```bash
docker run -p 5720:5720 -e OPENAI_API_KEY=sk-... stockyard/agentguard
```

## Part of Stockyard

AgentGuard is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AgentGuard standalone.
