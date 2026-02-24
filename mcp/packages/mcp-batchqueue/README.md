# @stockyard/mcp-batchqueue

> Background jobs for LLM calls

Async job queue for LLM requests with priority levels, concurrency control, and retry. Queue thousands of requests and process them reliably.

## Quick Start

```bash
npx @stockyard/mcp-batchqueue
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-batchqueue": {
      "command": "npx",
      "args": ["@stockyard/mcp-batchqueue"],
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
    "stockyard-batchqueue": {
      "command": "npx",
      "args": ["@stockyard/mcp-batchqueue"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `batchqueue_submit` | Submit a new LLM request to the queue |
| `batchqueue_status` | Get status of a queued job by ID |
| `batchqueue_result` | Get the result of a completed job |
| `batchqueue_queue_stats` | Get queue statistics: pending, processing, completed, failed |
| `batchqueue_cancel` | Cancel a pending or processing job |
| `batchqueue_proxy_status` | Check if the BatchQueue proxy is running and healthy |

## How It Works

1. On first run, downloads the `batchqueue` binary for your platform
2. Starts the BatchQueue proxy on port 5000
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:5000/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:5000/ui` for the real-time BatchQueue dashboard.

## Part of Stockyard

BatchQueue is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
