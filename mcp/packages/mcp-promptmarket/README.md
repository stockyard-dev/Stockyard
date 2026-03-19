# 🏪 @stockyard/mcp-promptmarket

**PromptMarket** — Community prompt library

Publish, browse, rate, fork prompts. Track which community prompts you use.

## Quick Start

```bash
npx @stockyard/mcp-promptmarket
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-promptmarket": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptmarket"],
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
| `promptmarket_setup` | Download and start the PromptMarket proxy |
| `promptmarket_stats` | Get marketplace stats. |
| `promptmarket_configure_client` | Get client configuration instructions |

## Part of Stockyard

PromptMarket is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
