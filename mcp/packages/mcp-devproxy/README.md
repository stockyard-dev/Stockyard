# 🔧 @stockyard/mcp-devproxy

**DevProxy** — Charles Proxy for LLM APIs

Interactive debugging proxy. Log headers, bodies, latency for every request. Development inspection tool.

## Quick Start

```bash
npx @stockyard/mcp-devproxy
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-devproxy": {
      "command": "npx",
      "args": ["@stockyard/mcp-devproxy"],
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
| `devproxy_setup` | Download and start the DevProxy proxy |
| `devproxy_stats` | Get debug stats: requests logged, avg latency. |
| `devproxy_recent` | List recent requests with headers and timing. |
| `devproxy_configure_client` | Get client configuration instructions |

## Part of Stockyard

DevProxy is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
