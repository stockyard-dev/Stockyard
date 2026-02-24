# 🏆 @stockyard/mcp-promptrank

**PromptRank** — Rank prompts by ROI

Per template: cost, quality, latency, volume, feedback. ROI leaderboard.

## Quick Start

```bash
npx @stockyard/mcp-promptrank
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-promptrank": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptrank"],
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
| `promptrank_setup` | Download and start the PromptRank proxy |
| `promptrank_stats` | Get prompt rankings. |
| `promptrank_configure_client` | Get client configuration instructions |

## Part of Stockyard

PromptRank is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
