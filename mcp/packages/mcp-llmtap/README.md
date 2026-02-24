# @stockyard/mcp-llmtap

> Full-stack LLM analytics in one binary

API analytics portal for LLM traffic. Latency percentiles (p50/p95/p99), error rates, cost breakdown by model, and volume tracking.

## Quick Start

```bash
npx @stockyard/mcp-llmtap
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-llmtap": {
      "command": "npx",
      "args": ["@stockyard/mcp-llmtap"],
      "env": {
        "OPENAI_API_KEY": "your-key-here"
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
    "stockyard-llmtap": {
      "command": "npx",
      "args": ["@stockyard/mcp-llmtap"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `llmtap_overview` | Get analytics overview: total requests, latency percentiles, error rate, cost |
| `llmtap_latency` | Get detailed latency breakdown: p50, p95, p99 by model |
| `llmtap_errors` | Get error breakdown by type, model, and time window |
| `llmtap_costs` | Get cost analytics: spend per model, per endpoint, trends |
| `llmtap_volume` | Get request volume over time for charting |
| `llmtap_proxy_status` | Check if the LLMTap proxy is running and healthy |

## How It Works

1. On first run, downloads the `llmtap` binary for your platform
2. Starts the LLMTap proxy on port 5300
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:5300/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:5300/ui` for the real-time LLMTap dashboard.

## Part of Stockyard

LLMTap is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
