# 🔄 @stockyard/mcp-streamtransform

**StreamTransform** — Transform streaming responses mid-stream

Pipeline on chunks: strip markdown, redact PII, translate. Minimal latency.

## Quick Start

```bash
npx @stockyard/mcp-streamtransform
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-streamtransform": {
      "command": "npx",
      "args": ["@stockyard/mcp-streamtransform"],
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
| `streamtransform_setup` | Download and start the StreamTransform proxy |
| `streamtransform_stats` | Get transform stats. |
| `streamtransform_configure_client` | Get client configuration instructions |

## Part of Stockyard

StreamTransform is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
