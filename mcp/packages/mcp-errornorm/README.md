# ⚠️ @stockyard/mcp-errornorm

**ErrorNorm** — Normalize error responses across providers

Single error schema: code, message, provider, retry_after, is_retryable.

## Quick Start

```bash
npx @stockyard/mcp-errornorm
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-errornorm": {
      "command": "npx",
      "args": ["@stockyard/mcp-errornorm"],
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
| `errornorm_setup` | Download and start the ErrorNorm proxy |
| `errornorm_stats` | Get error normalization stats. |
| `errornorm_configure_client` | Get client configuration instructions |

## Part of Stockyard

ErrorNorm is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
