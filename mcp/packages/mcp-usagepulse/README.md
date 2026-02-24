# @stockyard/mcp-usagepulse

> Know exactly where every token goes

Per-user and per-feature token metering. Multi-dimensional tracking, spend caps, and billing export in CSV/JSON.

## Quick Start

```bash
npx @stockyard/mcp-usagepulse
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-usagepulse": {
      "command": "npx",
      "args": ["@stockyard/mcp-usagepulse"],
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
    "stockyard-usagepulse": {
      "command": "npx",
      "args": ["@stockyard/mcp-usagepulse"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `usagepulse_usage` | Get usage breakdown by dimension (user, feature, team) |
| `usagepulse_user_usage` | Get usage for a specific user |
| `usagepulse_set_cap` | Set a spending cap for a user or team |
| `usagepulse_export` | Export usage data as CSV or JSON for billing |
| `usagepulse_proxy_status` | Check if the UsagePulse proxy is running and healthy |

## How It Works

1. On first run, downloads the `usagepulse` binary for your platform
2. Starts the UsagePulse proxy on port 4410
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4410/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4410/ui` for the real-time UsagePulse dashboard.

## Part of Stockyard

UsagePulse is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
