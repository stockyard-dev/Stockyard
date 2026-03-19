# 🗺️ @stockyard/mcp-costmap

**CostMap** — Multi-dimensional cost attribution

Tag requests with dimensions. Drill-down: by feature, user, prompt.

## Quick Start

```bash
npx @stockyard/mcp-costmap
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-costmap": {
      "command": "npx",
      "args": ["@stockyard/mcp-costmap"],
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
| `costmap_setup` | Download and start the CostMap proxy |
| `costmap_stats` | Get cost attribution stats. |
| `costmap_configure_client` | Get client configuration instructions |

## Part of Stockyard

CostMap is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
