# ♊ @stockyard/mcp-geminishim

**GeminiShim** — Tame Gemini's quirks behind clean API

Handle Gemini safety filter blocks with auto-retry. Normalize token counts. OpenAI-compatible surface for Gemini.

## Quick Start

```bash
npx @stockyard/mcp-geminishim
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-geminishim": {
      "command": "npx",
      "args": ["@stockyard/mcp-geminishim"],
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
| `geminishim_setup` | Download and start the GeminiShim proxy |
| `geminishim_stats` | Get Gemini compatibility stats: retries, safety blocks, normalizations. |
| `geminishim_configure_client` | Get client configuration instructions |

## Part of Stockyard

GeminiShim is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
