# 👶 @stockyard/mcp-agegate

**AgeGate** — Child safety middleware for LLM apps

Age-appropriate content filtering. Tiers: child, teen, adult. Injects safety prompts, filters output. COPPA/KOSA ready.

## Quick Start

```bash
npx @stockyard/mcp-agegate
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-agegate": {
      "command": "npx",
      "args": ["@stockyard/mcp-agegate"],
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
| `agegate_setup` | Download and start the AgeGate proxy |
| `agegate_stats` | Get safety stats: content filtered, tier distribution. |
| `agegate_configure_client` | Get client configuration instructions |

## Part of Stockyard

AgeGate is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
