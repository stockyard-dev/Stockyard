# 🎭 @stockyard/mcp-maskmode

**MaskMode** — Demo mode with realistic fake data

Replace real PII in responses with realistic fakes. Consistent within session. Perfect for sales demos.

## Quick Start

```bash
npx @stockyard/mcp-maskmode
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-maskmode": {
      "command": "npx",
      "args": ["@stockyard/mcp-maskmode"],
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
| `maskmode_setup` | Download and start the MaskMode proxy |
| `maskmode_stats` | Get masking stats: requests masked, replacements made. |
| `maskmode_configure_client` | Get client configuration instructions |

## Part of Stockyard

MaskMode is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
