# 💱 @stockyard/mcp-spotprice

**SpotPrice** — Real-time model pricing intelligence

Live pricing DB. Route to cheapest model meeting quality threshold.

## Quick Start

```bash
npx @stockyard/mcp-spotprice
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-spotprice": {
      "command": "npx",
      "args": ["@stockyard/mcp-spotprice"],
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
| `spotprice_setup` | Download and start the SpotPrice proxy |
| `spotprice_stats` | Get pricing stats. |
| `spotprice_configure_client` | Get client configuration instructions |

## Part of Stockyard

SpotPrice is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
