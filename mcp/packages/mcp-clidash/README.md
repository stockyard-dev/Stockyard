# 🖥️ @stockyard/mcp-clidash

**CliDash** — Terminal dashboard — htop for your LLM stack

Real-time TUI: req/sec, models, cache, spend, errors. SSH-accessible.

## Quick Start

```bash
npx @stockyard/mcp-clidash
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-clidash": {
      "command": "npx",
      "args": ["@stockyard/mcp-clidash"],
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
| `clidash_setup` | Download and start the CliDash proxy |
| `clidash_stats` | Get dashboard data. |
| `clidash_configure_client` | Get client configuration instructions |

## Part of Stockyard

CliDash is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
