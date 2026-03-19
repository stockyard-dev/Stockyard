# 🔑 @stockyard/mcp-authgate

**AuthGate** — API key management for YOUR users

Issue/revoke keys to your customers. Per-key limits and usage tracking.

## Quick Start

```bash
npx @stockyard/mcp-authgate
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-authgate": {
      "command": "npx",
      "args": ["@stockyard/mcp-authgate"],
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
| `authgate_setup` | Download and start the AuthGate proxy |
| `authgate_stats` | Get auth stats. |
| `authgate_keys` | List API keys. |
| `authgate_configure_client` | Get client configuration instructions |

## Part of Stockyard

AuthGate is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
