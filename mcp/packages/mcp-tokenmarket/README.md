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

TokenMarket is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
