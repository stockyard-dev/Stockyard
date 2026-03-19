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

ToolShield is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
