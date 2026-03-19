# 🏷️ @stockyard/mcp-whitelabel

**WhiteLabel** — Your brand on Stockyard's engine

Custom branding for resellers. Logo, colors, domain. Sell LLM infrastructure under your own brand.

## Quick Start

```bash
npx @stockyard/mcp-whitelabel
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-whitelabel": {
      "command": "npx",
      "args": ["@stockyard/mcp-whitelabel"],
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
| `whitelabel_setup` | Download and start the WhiteLabel proxy |
| `whitelabel_stats` | Get branding stats. |
| `whitelabel_configure_client` | Get client configuration instructions |

## Part of Stockyard

WhiteLabel is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
