# 🎨 @stockyard/mcp-imageproxy

**ImageProxy** — Proxy magic for image generation APIs

Cost tracking, caching, and failover for DALL-E and other image generation APIs.

## Quick Start

```bash
npx @stockyard/mcp-imageproxy
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-imageproxy": {
      "command": "npx",
      "args": ["@stockyard/mcp-imageproxy"],
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
| `imageproxy_setup` | Download and start the ImageProxy proxy |
| `imageproxy_stats` | Get image proxy stats: requests, cache hits, cost. |
| `imageproxy_configure_client` | Get client configuration instructions |

## Part of Stockyard

ImageProxy is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
