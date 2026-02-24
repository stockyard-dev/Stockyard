# 💱 @stockyard/mcp-geoprice

**GeoPrice** — Purchasing power pricing by region

PPP-adjusted pricing. Anti-VPN. Revenue by region dashboard.

## Quick Start

```bash
npx @stockyard/mcp-geoprice
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-geoprice": {
      "command": "npx",
      "args": ["@stockyard/mcp-geoprice"],
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
| `geoprice_setup` | Download and start the GeoPrice proxy |
| `geoprice_stats` | Get pricing stats. |
| `geoprice_configure_client` | Get client configuration instructions |

## Part of Stockyard

GeoPrice is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
