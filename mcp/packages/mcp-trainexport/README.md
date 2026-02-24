# 📤 @stockyard/mcp-trainexport

**TrainExport** — Export LLM conversations as fine-tuning datasets

Collect input/output pairs from live traffic. Export as OpenAI JSONL, Anthropic, or Alpaca format.

## Quick Start

```bash
npx @stockyard/mcp-trainexport
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-trainexport": {
      "command": "npx",
      "args": ["@stockyard/mcp-trainexport"],
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
| `trainexport_setup` | Download and start the TrainExport proxy |
| `trainexport_stats` | Get collection stats: pairs collected, storage used. |
| `trainexport_export` | Export collected pairs in specified format. |
| `trainexport_configure_client` | Get client configuration instructions |

## Part of Stockyard

TrainExport is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
