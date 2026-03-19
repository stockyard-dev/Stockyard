# 🔄 @stockyard/mcp-llmsync

**LLMSync** — Replicate config across environments

Environment hierarchy with config inheritance. Diff, promote, rollback. Git-friendly YAML management.

## Quick Start

```bash
npx @stockyard/mcp-llmsync
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-llmsync": {
      "command": "npx",
      "args": ["@stockyard/mcp-llmsync"],
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
| `llmsync_setup` | Download and start the LLMSync proxy |
| `llmsync_stats` | Get sync stats. |
| `llmsync_configure_client` | Get client configuration instructions |

## Part of Stockyard

LLMSync is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
