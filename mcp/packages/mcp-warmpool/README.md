# 🔥 @stockyard/mcp-warmpool

**WarmPool** — Pre-warm model connections

Persistent connections. Health checks. Keep-alive for Ollama.

## Quick Start

```bash
npx @stockyard/mcp-warmpool
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-warmpool": {
      "command": "npx",
      "args": ["@stockyard/mcp-warmpool"],
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
| `warmpool_setup` | Download and start the WarmPool proxy |
| `warmpool_stats` | Get connection pool stats. |
| `warmpool_configure_client` | Get client configuration instructions |

## Part of Stockyard

WarmPool is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
