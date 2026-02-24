# 🏗️ @stockyard/mcp-clustermode

**ClusterMode** — Run multiple instances with shared state

Multi-instance coordination. Leader-follower with shared cache. Scale beyond single-instance SQLite.

## Quick Start

```bash
npx @stockyard/mcp-clustermode
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-clustermode": {
      "command": "npx",
      "args": ["@stockyard/mcp-clustermode"],
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
| `clustermode_setup` | Download and start the ClusterMode proxy |
| `clustermode_stats` | Get cluster stats: nodes, requests distributed. |
| `clustermode_nodes` | List cluster nodes and their status. |
| `clustermode_configure_client` | Get client configuration instructions |

## Part of Stockyard

ClusterMode is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
