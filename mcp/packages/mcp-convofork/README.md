# 🌿 @stockyard/mcp-convofork

**ConvoFork** — Branch conversations — try different paths

Fork at any message. Independent history per branch. Tree visualization.

## Quick Start

```bash
npx @stockyard/mcp-convofork
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-convofork": {
      "command": "npx",
      "args": ["@stockyard/mcp-convofork"],
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
| `convofork_setup` | Download and start the ConvoFork proxy |
| `convofork_stats` | Get fork stats. |
| `convofork_configure_client` | Get client configuration instructions |

## Part of Stockyard

ConvoFork is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
