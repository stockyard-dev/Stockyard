# ConvoFork

**Branch conversations. Try different paths.**

ConvoFork lets users branch conversations at any message. Each fork has independent history. Tree visualization in dashboard.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/convofork

# Your app:   http://localhost:6200/v1/chat/completions
# Dashboard:  http://localhost:6200/ui
```

## What You Get

- Fork at any message
- Independent history per branch
- Tree visualization
- Compare branch outcomes
- Merge branches
- API for fork management

## Config

```yaml
# convofork.yaml
port: 6200
convofork:
  max_forks_per_session: 10
  max_depth: 5
```

## Docker

```bash
docker run -p 6200:6200 -e OPENAI_API_KEY=sk-... stockyard/convofork
```

## Part of Stockyard

ConvoFork is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ConvoFork standalone.
