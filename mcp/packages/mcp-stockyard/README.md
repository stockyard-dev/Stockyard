# @stockyard/mcp-stockyard

> The complete LLM infrastructure suite

All 125 Stockyard products in one binary. Cost control, caching, validation, routing, security, analytics, and more.

## Quick Start

```bash
npx @stockyard/mcp-stockyard
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-stockyard": {
      "command": "npx",
      "args": ["@stockyard/mcp-stockyard"],
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
    "stockyard-stockyard": {
      "command": "npx",
      "args": ["@stockyard/mcp-stockyard"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `stockyard_status` | Get full suite status: all enabled features, health, and summary stats |
| `stockyard_spend` | Get current LLM spending across all projects |
| `stockyard_cache_stats` | Get cache hit/miss statistics and savings |
| `stockyard_providers` | Get health and routing status of all providers |
| `stockyard_analytics` | Get analytics overview: latency, error rates, costs, volume |
| `stockyard_logs` | List recent LLM request logs |
| `stockyard_proxy_status` | Check if the Stockyard suite is running and healthy |

## How It Works

1. On first run, downloads the `stockyard` binary for your platform
2. Starts the Stockyard proxy on port 4000
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4000/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4000/ui` for the real-time Stockyard dashboard.

## Part of Stockyard

Stockyard is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
