# 🔱 @stockyard/mcp-streamsplit

**StreamSplit** — Fork streaming responses to multiple destinations

Tee SSE chunks to logger, quality checker, webhook. Zero latency for primary.

## Quick Start

```bash
npx @stockyard/mcp-streamsplit
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-streamsplit": {
      "command": "npx",
      "args": ["@stockyard/mcp-streamsplit"],
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
| `streamsplit_setup` | Download and start the StreamSplit proxy |
| `streamsplit_stats` | Get stream split stats. |
| `streamsplit_configure_client` | Get client configuration instructions |

## Part of Stockyard

StreamSplit is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
