# 📄 @stockyard/mcp-docparse

**DocParse** — Preprocess documents before they hit the LLM

PDF/Word/HTML text extraction. Smart chunking. Clean artifacts.

## Quick Start

```bash
npx @stockyard/mcp-docparse
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-docparse": {
      "command": "npx",
      "args": ["@stockyard/mcp-docparse"],
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
| `docparse_setup` | Download and start the DocParse proxy |
| `docparse_stats` | Get document processing stats. |
| `docparse_configure_client` | Get client configuration instructions |

## Part of Stockyard

DocParse is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
