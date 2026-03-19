# 🎯 @stockyard/mcp-scopeguard

**ScopeGuard** — Fine-grained permissions per API key

Role-based access control. Map keys to allowed models, endpoints, features.

## Quick Start

```bash
npx @stockyard/mcp-scopeguard
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-scopeguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-scopeguard"],
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
| `scopeguard_setup` | Download and start the ScopeGuard proxy |
| `scopeguard_stats` | Get permission stats. |
| `scopeguard_roles` | List roles. |
| `scopeguard_configure_client` | Get client configuration instructions |

## Part of Stockyard

ScopeGuard is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
