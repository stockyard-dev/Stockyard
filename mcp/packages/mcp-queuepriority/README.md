# 👑 @stockyard/mcp-queuepriority

**QueuePriority** — Priority queues — VIP users first

Priority levels per key/tenant. Reserved capacity. SLA tracking.

## Quick Start

```bash
npx @stockyard/mcp-queuepriority
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-queuepriority": {
      "command": "npx",
      "args": ["@stockyard/mcp-queuepriority"],
      "env": {
        "OPENAI_API_KEY": "your-key"
      }
    }
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `queuepriority_setup` | Download and start the QueuePriority proxy |
| `queuepriority_stats` | Get queue stats. |
| `queuepriority_configure_client` | Get client configuration instructions |

## Part of Stockyard

QueuePriority is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
