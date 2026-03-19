# 🔐 @stockyard/mcp-envsync

**EnvSync** — Sync configs + secrets across environments

Push/promote/diff. Encrypted secrets. Pre-promotion validation. Rollback.

## Quick Start

```bash
npx @stockyard/mcp-envsync
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-envsync": {
      "command": "npx",
      "args": ["@stockyard/mcp-envsync"],
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
| `envsync_setup` | Download and start the EnvSync proxy |
| `envsync_stats` | Get sync stats. |
| `envsync_configure_client` | Get client configuration instructions |

## Part of Stockyard

EnvSync is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
