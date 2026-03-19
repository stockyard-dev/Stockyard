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

ParamNorm is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
