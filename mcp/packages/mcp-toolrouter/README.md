# 🔀 @stockyard/mcp-toolrouter

**ToolRouter** — Manage, version, and route LLM function calls

Versioned tool schemas. Route calls. Shadow-test. Usage analytics.

## Quick Start

```bash
npx @stockyard/mcp-toolrouter
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-toolrouter": {
      "command": "npx",
      "args": ["@stockyard/mcp-toolrouter"],
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
| `toolrouter_setup` | Download and start the ToolRouter proxy |
| `toolrouter_stats` | Get tool routing stats. |
| `toolrouter_tools` | List registered tools. |
| `toolrouter_configure_client` | Get client configuration instructions |

## Part of Stockyard

ToolRouter is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
