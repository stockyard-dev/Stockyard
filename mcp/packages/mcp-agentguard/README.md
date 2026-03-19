# 🛡️ @stockyard/mcp-agentguard

**AgentGuard** — Safety rails for autonomous AI agents

Per-session limits for AI agents: max calls, cost, duration. Kill runaway agent sessions before they drain your budget.

## Quick Start

```bash
npx @stockyard/mcp-agentguard
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-agentguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-agentguard"],
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
| `agentguard_setup` | Download and start the AgentGuard proxy |
| `agentguard_sessions` | List active agent sessions with call counts, cost, and duration. |
| `agentguard_kill` | Kill a specific agent session by ID. |
| `agentguard_stats` | Get aggregate stats: sessions tracked, killed, costs saved. |
| `agentguard_configure_client` | Get client configuration instructions |

## Part of Stockyard

AgentGuard is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
