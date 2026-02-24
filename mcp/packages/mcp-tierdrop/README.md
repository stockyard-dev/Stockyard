# 📉 @stockyard/mcp-tierdrop

**TierDrop** — Auto-downgrade models when burning cash

Gracefully degrade from GPT-4 to GPT-3.5 when approaching budget limits. Cost-aware model selection.

## Quick Start

```bash
npx @stockyard/mcp-tierdrop
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-tierdrop": {
      "command": "npx",
      "args": ["@stockyard/mcp-tierdrop"],
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
| `tierdrop_setup` | Download and start the TierDrop proxy |
| `tierdrop_stats` | Get downgrade stats: triggers, models switched, savings. |
| `tierdrop_tiers` | List configured cost tiers and thresholds. |
| `tierdrop_configure_client` | Get client configuration instructions |

## Part of Stockyard

TierDrop is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
