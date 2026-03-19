# 🌐 @stockyard/mcp-edgecache

**EdgeCache** — CDN-like caching for LLM responses

Distribute cache across instances. Geographic hit rates.

## Quick Start

```bash
npx @stockyard/mcp-edgecache
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-edgecache": {
      "command": "npx",
      "args": ["@stockyard/mcp-edgecache"],
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
| `edgecache_setup` | Download and start the EdgeCache proxy |
| `edgecache_stats` | Get edge cache stats. |
| `edgecache_configure_client` | Get client configuration instructions |

## Part of Stockyard

EdgeCache is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
