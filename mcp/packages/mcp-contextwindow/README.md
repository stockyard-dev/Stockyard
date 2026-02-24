# 🪟 @stockyard/mcp-contextwindow

**ContextWindow** — Visual context window debugger

Visualize token allocation by message role. See what's eating your context window. Optimization recommendations.

## Quick Start

```bash
npx @stockyard/mcp-contextwindow
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-contextwindow": {
      "command": "npx",
      "args": ["@stockyard/mcp-contextwindow"],
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
| `contextwindow_setup` | Download and start the ContextWindow proxy |
| `contextwindow_stats` | Get context window analysis: breakdown by role, total usage. |
| `contextwindow_configure_client` | Get client configuration instructions |

## Part of Stockyard

ContextWindow is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
