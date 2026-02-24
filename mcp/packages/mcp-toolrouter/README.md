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

ToolRouter is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
