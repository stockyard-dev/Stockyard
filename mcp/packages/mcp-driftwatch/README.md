# 📊 @stockyard/mcp-driftwatch

**DriftWatch** — Detect when model behavior changes

Track latency and output patterns per model over time. Alert when behavior drifts beyond thresholds.

## Quick Start

```bash
npx @stockyard/mcp-driftwatch
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-driftwatch": {
      "command": "npx",
      "args": ["@stockyard/mcp-driftwatch"],
      "env": {
        "OPENAI_API_KEY": "your-key"
      }
    }
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `driftwatch_setup` | Download and start the DriftWatch proxy |
| `driftwatch_stats` | Get drift detection stats per model. |
| `driftwatch_baselines` | View current baselines for tracked models. |
| `driftwatch_configure_client` | Get client configuration instructions |

## Part of Stockyard

DriftWatch is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
