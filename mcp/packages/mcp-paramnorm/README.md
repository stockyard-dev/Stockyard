# ⚖️ @stockyard/mcp-paramnorm

**ParamNorm** — Normalize parameters across providers

Calibration profiles per model. Map normalized params to model-specific ranges.

## Quick Start

```bash
npx @stockyard/mcp-paramnorm
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-paramnorm": {
      "command": "npx",
      "args": ["@stockyard/mcp-paramnorm"],
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
| `paramnorm_setup` | Download and start the ParamNorm proxy |
| `paramnorm_stats` | Get normalization stats. |
| `paramnorm_configure_client` | Get client configuration instructions |

## Part of Stockyard

ParamNorm is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
