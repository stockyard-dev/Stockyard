# @stockyard/mcp-retrypilot

> Intelligent retries that actually work

Smart retry engine with exponential backoff, circuit breakers, deadline awareness, and automatic model downgrade on failures.

## Quick Start

```bash
npx @stockyard/mcp-retrypilot
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-retrypilot": {
      "command": "npx",
      "args": ["@stockyard/mcp-retrypilot"],
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
    "stockyard-retrypilot": {
      "command": "npx",
      "args": ["@stockyard/mcp-retrypilot"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `retrypilot_stats` | Get retry statistics: total retries, success rate, avg retries per request |
| `retrypilot_circuit_status` | Get circuit breaker status for each model: closed, open, or half-open |
| `retrypilot_reset_circuit` | Manually reset a tripped circuit breaker |
| `retrypilot_set_config` | Update retry configuration dynamically |
| `retrypilot_budget` | Get retry budget status: retries used this minute, remaining |
| `retrypilot_proxy_status` | Check if the RetryPilot proxy is running and healthy |

## How It Works

1. On first run, downloads the `retrypilot` binary for your platform
2. Starts the RetryPilot proxy on port 5500
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:5500/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:5500/ui` for the real-time RetryPilot dashboard.

## Part of Stockyard

RetryPilot is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
