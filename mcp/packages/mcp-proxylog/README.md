# 📋 @stockyard/mcp-proxylog

**ProxyLog** — Structured logging for every proxy decision

Each middleware emits decision log. Per-request trace. X-Proxy-Trace header.

## Quick Start

```bash
npx @stockyard/mcp-proxylog
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-proxylog": {
      "command": "npx",
      "args": ["@stockyard/mcp-proxylog"],
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
| `proxylog_setup` | Download and start the ProxyLog proxy |
| `proxylog_stats` | Get logging stats. |
| `proxylog_configure_client` | Get client configuration instructions |

## Part of Stockyard

ProxyLog is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
