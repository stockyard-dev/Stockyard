# 🚦 @stockyard/mcp-streamthrottle

**StreamThrottle** — Control streaming speed for better UX

Max tokens/sec. Buffer fast streams. Per endpoint/model/client.

## Quick Start

```bash
npx @stockyard/mcp-streamthrottle
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-streamthrottle": {
      "command": "npx",
      "args": ["@stockyard/mcp-streamthrottle"],
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
| `streamthrottle_setup` | Download and start the StreamThrottle proxy |
| `streamthrottle_stats` | Get throttle stats. |
| `streamthrottle_configure_client` | Get client configuration instructions |

## Part of Stockyard

StreamThrottle is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
