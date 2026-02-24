# @stockyard/mcp-modelswitch

> Right model, right prompt, right price

Smart model routing based on token count, prompt patterns, and headers. Route complex queries to GPT-4o and simple ones to GPT-4o-mini.

## Quick Start

```bash
npx @stockyard/mcp-modelswitch
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-modelswitch": {
      "command": "npx",
      "args": ["@stockyard/mcp-modelswitch"],
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
    "stockyard-modelswitch": {
      "command": "npx",
      "args": ["@stockyard/mcp-modelswitch"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `modelswitch_stats` | Get routing statistics: requests per model, cost per route, A/B test results |
| `modelswitch_rules` | List current routing rules and their match counts |
| `modelswitch_add_rule` | Add a new routing rule |
| `modelswitch_test` | Test which model a prompt would be routed to |
| `modelswitch_proxy_status` | Check if the ModelSwitch proxy is running and healthy |

## How It Works

1. On first run, downloads the `modelswitch` binary for your platform
2. Starts the ModelSwitch proxy on port 4900
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4900/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4900/ui` for the real-time ModelSwitch dashboard.

## Part of Stockyard

ModelSwitch is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
