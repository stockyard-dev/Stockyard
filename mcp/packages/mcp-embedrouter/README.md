# 🔀 @stockyard/mcp-embedrouter

**EmbedRouter** — Smart routing for embedding requests

Batch over 50ms window. Deduplicate. Route by content type.

## Quick Start

```bash
npx @stockyard/mcp-embedrouter
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-embedrouter": {
      "command": "npx",
      "args": ["@stockyard/mcp-embedrouter"],
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
| `embedrouter_setup` | Download and start the EmbedRouter proxy |
| `embedrouter_stats` | Get embedding routing stats. |
| `embedrouter_configure_client` | Get client configuration instructions |

## Part of Stockyard

EmbedRouter is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
