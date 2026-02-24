# 🃏 @stockyard/mcp-toolmock

**ToolMock** — Fake tool responses for testing

Canned responses by tool+args. Simulate errors, timeouts, partial results.

## Quick Start

```bash
npx @stockyard/mcp-toolmock
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-toolmock": {
      "command": "npx",
      "args": ["@stockyard/mcp-toolmock"],
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
| `toolmock_setup` | Download and start the ToolMock proxy |
| `toolmock_stats` | Get mock stats. |
| `toolmock_configure_client` | Get client configuration instructions |

## Part of Stockyard

ToolMock is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
