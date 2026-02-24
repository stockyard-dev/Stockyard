# AgentReplay

**Record and replay agent sessions.**

AgentReplay reconstructs full agent sessions from TraceLink data. Step-by-step playback with what-if mode.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/agentreplay

# Your app:   http://localhost:6530/v1/chat/completions
# Dashboard:  http://localhost:6530/ui
```

## What You Get

- Session reconstruction from traces
- Step-by-step playback
- What-if mode (change a step, replay)
- Export as test cases
- Decision tree visualization
- Dashboard with session explorer

## Config

```yaml
# agentreplay.yaml
port: 6530
agentreplay:
  source: tracelink
  max_session_length: 100
  export_format: jsonl
```

## Docker

```bash
docker run -p 6530:6530 -e OPENAI_API_KEY=sk-... stockyard/agentreplay
```

## Part of Stockyard

AgentReplay is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AgentReplay standalone.
