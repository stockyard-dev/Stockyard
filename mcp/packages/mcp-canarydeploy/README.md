# 🐤 @stockyard/mcp-canarydeploy

**CanaryDeploy** — Canary deployments for prompt/model changes

Gradual rollout: 5%→25%→100%. Auto-promote if quality holds. Auto-rollback.

## Quick Start

```bash
npx @stockyard/mcp-canarydeploy
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-canarydeploy": {
      "command": "npx",
      "args": ["@stockyard/mcp-canarydeploy"],
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
| `canarydeploy_setup` | Download and start the CanaryDeploy proxy |
| `canarydeploy_stats` | Get canary deployment stats. |
| `canarydeploy_configure_client` | Get client configuration instructions |

## Part of Stockyard

CanaryDeploy is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
