# 🔮 @stockyard/mcp-costpredict

**CostPredict** — Predict request cost BEFORE sending

Count input tokens. Estimate output. Calculate cost. X-Estimated-Cost header.

## Quick Start

```bash
npx @stockyard/mcp-costpredict
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-costpredict": {
      "command": "npx",
      "args": ["@stockyard/mcp-costpredict"],
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
| `costpredict_setup` | Download and start the CostPredict proxy |
| `costpredict_stats` | Get prediction stats. |
| `costpredict_configure_client` | Get client configuration instructions |

## Part of Stockyard

CostPredict is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
