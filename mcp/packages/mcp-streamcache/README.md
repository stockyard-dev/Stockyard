# 📺 @stockyard/mcp-streamcache

**StreamCache** — Cache streaming responses with realistic timing

Store original chunk timing. Replay cached SSE with original pacing.

## Quick Start

```bash
npx @stockyard/mcp-streamcache
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-streamcache": {
      "command": "npx",
      "args": ["@stockyard/mcp-streamcache"],
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
| `streamcache_setup` | Download and start the StreamCache proxy |
| `streamcache_stats` | Get stream cache stats. |
| `streamcache_configure_client` | Get client configuration instructions |

## Part of Stockyard

StreamCache is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
