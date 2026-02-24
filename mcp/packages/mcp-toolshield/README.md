# 🛡️ @stockyard/mcp-toolshield

**ToolShield** — Validate and sandbox LLM tool calls

Intercept tool_use. Validate args. Per-tool permissions and rate limits.

## Quick Start

```bash
npx @stockyard/mcp-toolshield
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-toolshield": {
      "command": "npx",
      "args": ["@stockyard/mcp-toolshield"],
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
| `toolshield_setup` | Download and start the ToolShield proxy |
| `toolshield_stats` | Get tool validation stats. |
| `toolshield_configure_client` | Get client configuration instructions |

## Part of Stockyard

ToolShield is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
