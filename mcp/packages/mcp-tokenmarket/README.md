# 🏪 @stockyard/mcp-tokenmarket

**TokenMarket** — Dynamic budget reallocation across teams

Pool-based budgets. Teams request capacity. Auto-rebalance. Priority queuing for high-value requests.

## Quick Start

```bash
npx @stockyard/mcp-tokenmarket
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-tokenmarket": {
      "command": "npx",
      "args": ["@stockyard/mcp-tokenmarket"],
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
| `tokenmarket_setup` | Download and start the TokenMarket proxy |
| `tokenmarket_stats` | Get market stats: pool balances, transactions. |
| `tokenmarket_pools` | List budget pools with current balances. |
| `tokenmarket_configure_client` | Get client configuration instructions |

## Part of Stockyard

TokenMarket is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
