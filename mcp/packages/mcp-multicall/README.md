# @stockyard/mcp-multicall

> Ask multiple models, pick the best answer

Send the same prompt to multiple LLMs simultaneously. Pick the fastest, cheapest, longest, or consensus response.

## Quick Start

```bash
npx @stockyard/mcp-multicall
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-multicall": {
      "command": "npx",
      "args": ["@stockyard/mcp-multicall"],
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
    "stockyard-multicall": {
      "command": "npx",
      "args": ["@stockyard/mcp-multicall"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `multicall_stats` | Get multi-call statistics: wins per model, avg latency, cost comparison |
| `multicall_routes` | List configured multi-call routes and their strategies |
| `multicall_add_route` | Add a new multi-call route |
| `multicall_compare` | Send a prompt to all models and return all responses for comparison |
| `multicall_proxy_status` | Check if the MultiCall proxy is running and healthy |

## How It Works

1. On first run, downloads the `multicall` binary for your platform
2. Starts the MultiCall proxy on port 5100
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:5100/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:5100/ui` for the real-time MultiCall dashboard.

## Part of Stockyard

MultiCall is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
