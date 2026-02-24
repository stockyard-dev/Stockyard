# 🌐 @stockyard/mcp-langbridge

**LangBridge** — Cross-language translation for multilingual apps

Auto-detect language, translate to English for model, translate response back. Seamless multilingual support.

## Quick Start

```bash
npx @stockyard/mcp-langbridge
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-langbridge": {
      "command": "npx",
      "args": ["@stockyard/mcp-langbridge"],
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
| `langbridge_setup` | Download and start the LangBridge proxy |
| `langbridge_stats` | Get translation stats: languages detected, translations performed. |
| `langbridge_configure_client` | Get client configuration instructions |

## Part of Stockyard

LangBridge is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
