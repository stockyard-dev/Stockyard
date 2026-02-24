# 📡 @stockyard/mcp-anomalyradar

**AnomalyRadar** — ML-powered anomaly detection

Build statistical baselines. Z-score deviation detection. Auto-adjusting thresholds.

## Quick Start

```bash
npx @stockyard/mcp-anomalyradar
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-anomalyradar": {
      "command": "npx",
      "args": ["@stockyard/mcp-anomalyradar"],
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
| `anomalyradar_setup` | Download and start the AnomalyRadar proxy |
| `anomalyradar_stats` | Get anomaly detection stats. |
| `anomalyradar_configure_client` | Get client configuration instructions |

## Part of Stockyard

AnomalyRadar is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
