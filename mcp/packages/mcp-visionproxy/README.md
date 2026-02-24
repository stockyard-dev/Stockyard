# 👁️ @stockyard/mcp-visionproxy

**VisionProxy** — Proxy magic for vision/image APIs

Caching, cost tracking, and failover for GPT-4V, Claude vision.

## Quick Start

```bash
npx @stockyard/mcp-visionproxy
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-visionproxy": {
      "command": "npx",
      "args": ["@stockyard/mcp-visionproxy"],
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
| `visionproxy_setup` | Download and start the VisionProxy proxy |
| `visionproxy_stats` | Get vision proxy stats. |
| `visionproxy_configure_client` | Get client configuration instructions |

## Part of Stockyard

VisionProxy is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
