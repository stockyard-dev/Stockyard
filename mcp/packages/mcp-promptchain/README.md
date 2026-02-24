# 🔗 @stockyard/mcp-promptchain

**PromptChain** — Composable prompt blocks

Define reusable blocks. Compose: [tone.helpful, format.json, domain.ecommerce]. Auto-update.

## Quick Start

```bash
npx @stockyard/mcp-promptchain
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-promptchain": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptchain"],
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
| `promptchain_setup` | Download and start the PromptChain proxy |
| `promptchain_stats` | Get composition stats. |
| `promptchain_blocks` | List defined blocks. |
| `promptchain_configure_client` | Get client configuration instructions |

## Part of Stockyard

PromptChain is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
