# @stockyard/mcp-alertpulse

> PagerDuty for your LLM stack

**Configurable alerting for LLM infrastructure via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-alertpulse": {
      "command": "npx",
      "args": ["@stockyard/mcp-alertpulse"],
      "env": {
        "OPENAI_API_KEY": "sk-your-key-here"
      }
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "stockyard-alertpulse": {
      "command": "npx",
      "args": ["@stockyard/mcp-alertpulse"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-alertpulse": {
      "command": "npx",
      "args": ["@stockyard/mcp-alertpulse"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up AlertPulse"** — Downloads and starts the proxy
- **"List all alert rules with current status (firing/OK)"**
- **"Add a new alert rule"**
- **"Get alert history: recent firings and resolutions"**
- **"Fire a test alert to verify notification channels work"**
- **"Check if the AlertPulse proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `alertpulse` binary for your platform
2. It writes a config and starts the proxy on port 5640
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5640/v1` to route through AlertPulse
5. Dashboard available at `http://127.0.0.1:5640/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why AlertPulse?

Configurable alerting for LLM infrastructure. Rules for error rates, latency, cost thresholds. Notify via Slack, Discord, PagerDuty, email, or webhooks.

## Part of Stockyard

AlertPulse is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
