# Stockyard × n8n

> LLM cost caps, caching, and analytics as an n8n node.

## Install

```bash
npm install n8n-nodes-stockyard
```

Or in n8n: Settings → Community Nodes → Install → `n8n-nodes-stockyard`

## Operations

| Operation | Description |
|-----------|-------------|
| Chat Completion | Send chat requests through Stockyard proxy |
| Get Spend | Check current spending and budget |
| Cache Stats | View cache hit rate and savings |
| Flush Cache | Clear cached responses |
| Provider Status | Check health of all LLM providers |
| Analytics | Usage overview with latency and error rates |
| Health Check | Verify Stockyard proxy is running |

## Setup

1. Start Stockyard: `npx @stockyard/stockyard`
2. In n8n: Add credentials → Stockyard API → URL: `http://localhost:4000`
3. Add Stockyard node to your workflow

## License

MIT
