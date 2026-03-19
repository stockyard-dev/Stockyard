# 🌍 @stockyard/mcp-regionroute

**RegionRoute** — Data residency routing for GDPR compliance

Route requests to region-specific endpoints. Keep EU data in EU. Geographic compliance made easy.

## Quick Start

```bash
npx @stockyard/mcp-regionroute
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-regionroute": {
      "command": "npx",
      "args": ["@stockyard/mcp-regionroute"],
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
| `regionroute_setup` | Download and start the RegionRoute proxy |
| `regionroute_stats` | Get routing stats: requests per region. |
| `regionroute_routes` | List configured region routes. |
| `regionroute_configure_client` | Get client configuration instructions |

## Part of Stockyard

RegionRoute is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
