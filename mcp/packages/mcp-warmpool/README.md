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

WarmPool is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
