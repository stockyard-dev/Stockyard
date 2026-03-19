# 💬 @stockyard/mcp-sessionstore

**SessionStore** — Managed conversation sessions

Create/resume/list/delete sessions. Full history. Metadata. Concurrent limits.

## Quick Start

```bash
npx @stockyard/mcp-sessionstore
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-sessionstore": {
      "command": "npx",
      "args": ["@stockyard/mcp-sessionstore"],
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
| `sessionstore_setup` | Download and start the SessionStore proxy |
| `sessionstore_stats` | Get session stats. |
| `sessionstore_sessions` | List active sessions. |
| `sessionstore_configure_client` | Get client configuration instructions |

## Part of Stockyard

SessionStore is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
