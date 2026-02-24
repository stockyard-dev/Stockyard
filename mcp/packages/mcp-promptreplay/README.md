# @stockyard/mcp-promptreplay

> Every LLM call, logged and replayable

Full request/response logging for LLM APIs. Capture every prompt, completion, and token count. Replay past requests for debugging.

## Quick Start

```bash
npx @stockyard/mcp-promptreplay
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-promptreplay": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptreplay"],
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
    "stockyard-promptreplay": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptreplay"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `promptreplay_list` | List recent logged LLM requests with timestamps, models, and costs |
| `promptreplay_get` | Get full details of a logged request including prompt and response |
| `promptreplay_replay` | Replay a previously logged LLM request |
| `promptreplay_stats` | Get logging statistics: total entries, storage size, requests by model |
| `promptreplay_proxy_status` | Check if the PromptReplay proxy is running and healthy |

## How It Works

1. On first run, downloads the `promptreplay` binary for your platform
2. Starts the PromptReplay proxy on port 4600
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4600/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4600/ui` for the real-time PromptReplay dashboard.

## Part of Stockyard

PromptReplay is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
