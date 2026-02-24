# @stockyard/mcp-tokentrim

> Never hit a context limit again

Automatic context window management. Truncates prompts using middle-out, oldest-first, or newest-first strategies when they exceed model limits.

## Quick Start

```bash
npx @stockyard/mcp-tokentrim
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-tokentrim": {
      "command": "npx",
      "args": ["@stockyard/mcp-tokentrim"],
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
    "stockyard-tokentrim": {
      "command": "npx",
      "args": ["@stockyard/mcp-tokentrim"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `tokentrim_stats` | Get trimming statistics: total trimmed, tokens saved, trim rate |
| `tokentrim_count` | Count tokens in a text string without sending to LLM |
| `tokentrim_set_strategy` | Change the truncation strategy |
| `tokentrim_set_limit` | Set a custom token limit for a model |
| `tokentrim_proxy_status` | Check if the TokenTrim proxy is running and healthy |

## How It Works

1. On first run, downloads the `tokentrim` binary for your platform
2. Starts the TokenTrim proxy on port 4901
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4901/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4901/ui` for the real-time TokenTrim dashboard.

## Part of Stockyard

TokenTrim is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
