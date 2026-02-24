# 📏 @stockyard/mcp-outputcap

**OutputCap** — Stop paying for responses you don't need

Cap output length at natural sentence boundaries. No more 500-token essays when you asked for one word.

## Quick Start

```bash
npx @stockyard/mcp-outputcap
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-outputcap": {
      "command": "npx",
      "args": ["@stockyard/mcp-outputcap"],
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
| `outputcap_setup` | Download and start the OutputCap proxy |
| `outputcap_stats` | Get capping stats: tokens saved, avg reduction. |
| `outputcap_configure_client` | Get client configuration instructions |

## Part of Stockyard

OutputCap is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
