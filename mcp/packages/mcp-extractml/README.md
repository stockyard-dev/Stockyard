# 🧲 @stockyard/mcp-extractml

**ExtractML** — Turn unstructured LLM responses into structured data

Force extraction from free-text into JSON when models return prose.

## Quick Start

```bash
npx @stockyard/mcp-extractml
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-extractml": {
      "command": "npx",
      "args": ["@stockyard/mcp-extractml"],
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
| `extractml_setup` | Download and start the ExtractML proxy |
| `extractml_stats` | Get extraction stats. |
| `extractml_configure_client` | Get client configuration instructions |

## Part of Stockyard

ExtractML is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
