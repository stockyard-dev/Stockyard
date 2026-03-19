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

VisionProxy is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
