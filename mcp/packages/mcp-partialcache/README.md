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

PartialCache is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
