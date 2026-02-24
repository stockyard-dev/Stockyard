# 📈 @stockyard/mcp-quotasync

**QuotaSync** — Track provider rate limits in real-time

Parse rate limit headers. Track per model/endpoint. Alert near limits.

## Quick Start

```bash
npx @stockyard/mcp-quotasync
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-quotasync": {
      "command": "npx",
      "args": ["@stockyard/mcp-quotasync"],
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
| `quotasync_setup` | Download and start the QuotaSync proxy |
| `quotasync_stats` | Get quota tracking stats. |
| `quotasync_configure_client` | Get client configuration instructions |

## Part of Stockyard

QuotaSync is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
