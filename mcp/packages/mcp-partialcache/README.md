# 🧩 @stockyard/mcp-partialcache

**PartialCache** — Cache reusable prompt prefixes

Detect static system prompt prefix. Use native prefix caching where supported.

## Quick Start

```bash
npx @stockyard/mcp-partialcache
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-partialcache": {
      "command": "npx",
      "args": ["@stockyard/mcp-partialcache"],
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
| `partialcache_setup` | Download and start the PartialCache proxy |
| `partialcache_stats` | Get prefix cache stats. |
| `partialcache_configure_client` | Get client configuration instructions |

## Part of Stockyard

PartialCache is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
