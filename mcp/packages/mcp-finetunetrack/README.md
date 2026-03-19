# 📉 @stockyard/mcp-finetunetrack

**FineTuneTrack** — Monitor fine-tuned model performance

Eval suite. Run periodically. Track scores. Compare to base model.

## Quick Start

```bash
npx @stockyard/mcp-finetunetrack
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-finetunetrack": {
      "command": "npx",
      "args": ["@stockyard/mcp-finetunetrack"],
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
| `finetunetrack_setup` | Download and start the FineTuneTrack proxy |
| `finetunetrack_stats` | Get fine-tune tracking stats. |
| `finetunetrack_configure_client` | Get client configuration instructions |

## Part of Stockyard

FineTuneTrack is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
