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

PromptRank is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
