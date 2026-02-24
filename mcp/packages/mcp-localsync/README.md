# 🏠 @stockyard/mcp-localsync

**LocalSync** — Seamlessly blend local and cloud models

Route to Ollama locally when available. Auto-failover to cloud when local is down. Track cost savings.

## Quick Start

```bash
npx @stockyard/mcp-localsync
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-localsync": {
      "command": "npx",
      "args": ["@stockyard/mcp-localsync"],
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
| `localsync_setup` | Download and start the LocalSync proxy |
| `localsync_stats` | Get routing stats: local vs cloud, savings, failovers. |
| `localsync_health` | Check local endpoint health. |
| `localsync_configure_client` | Get client configuration instructions |

## Part of Stockyard

LocalSync is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
