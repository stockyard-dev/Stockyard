# 📝 @stockyard/mcp-summarizegate

**SummarizeGate** — Auto-summarize long contexts to save tokens

Score relevance per section. Keep high-relevance verbatim. Summarize low-relevance.

## Quick Start

```bash
npx @stockyard/mcp-summarizegate
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-summarizegate": {
      "command": "npx",
      "args": ["@stockyard/mcp-summarizegate"],
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
| `summarizegate_setup` | Download and start the SummarizeGate proxy |
| `summarizegate_stats` | Get summarization stats. |
| `summarizegate_configure_client` | Get client configuration instructions |

## Part of Stockyard

SummarizeGate is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
