# @stockyard/mcp-keypool

> Pool your API keys, multiply your limits

API key pooling and rotation for LLM providers. Round-robin, least-used, or random strategies. Auto-rotate on 429 rate limits.

## Quick Start

```bash
npx @stockyard/mcp-keypool
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-keypool": {
      "command": "npx",
      "args": ["@stockyard/mcp-keypool"],
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
    "stockyard-keypool": {
      "command": "npx",
      "args": ["@stockyard/mcp-keypool"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `keypool_status` | Get status of all pooled API keys: active, cooldown, usage counts, last error |
| `keypool_add_key` | Add a new API key to the pool |
| `keypool_remove_key` | Remove an API key from the pool by name |
| `keypool_set_strategy` | Change the key rotation strategy |
| `keypool_stats` | Get pool statistics: requests per key, 429 counts, rotation events |
| `keypool_proxy_status` | Check if the KeyPool proxy is running and healthy |

## How It Works

1. On first run, downloads the `keypool` binary for your platform
2. Starts the KeyPool proxy on port 4700
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4700/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4700/ui` for the real-time KeyPool dashboard.

## Part of Stockyard

KeyPool is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
