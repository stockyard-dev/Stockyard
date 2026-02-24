# @stockyard/mcp-streamsnap

> Capture and replay every LLM stream

SSE stream capture with zero latency overhead. Record TTFT, tokens/sec, and full responses. Replay captured streams for testing.

## Quick Start

```bash
npx @stockyard/mcp-streamsnap
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-streamsnap": {
      "command": "npx",
      "args": ["@stockyard/mcp-streamsnap"],
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
    "stockyard-streamsnap": {
      "command": "npx",
      "args": ["@stockyard/mcp-streamsnap"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `streamsnap_captures` | List recent stream captures with TTFT, tokens/sec, and metadata |
| `streamsnap_get` | Get full captured stream content by ID |
| `streamsnap_metrics` | Get aggregated streaming metrics: avg TTFT, avg tokens/sec, completion rate |
| `streamsnap_replay` | Replay a captured stream as if it were live |
| `streamsnap_proxy_status` | Check if the StreamSnap proxy is running and healthy |

## How It Works

1. On first run, downloads the `streamsnap` binary for your platform
2. Starts the StreamSnap proxy on port 5200
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:5200/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:5200/ui` for the real-time StreamSnap dashboard.

## Part of Stockyard

StreamSnap is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
