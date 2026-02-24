# 🏷️ @stockyard/mcp-tokenauction

**TokenAuction** — Dynamic pricing based on demand

Monitor costs, queue, errors. Time-of-day pricing. Surge pricing.

## Quick Start

```bash
npx @stockyard/mcp-tokenauction
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-tokenauction": {
      "command": "npx",
      "args": ["@stockyard/mcp-tokenauction"],
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
| `tokenauction_setup` | Download and start the TokenAuction proxy |
| `tokenauction_stats` | Get auction stats. |
| `tokenauction_configure_client` | Get client configuration instructions |

## Part of Stockyard

TokenAuction is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
