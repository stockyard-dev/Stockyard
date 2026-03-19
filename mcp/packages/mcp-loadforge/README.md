# ⚡ @stockyard/mcp-loadforge

**LoadForge** — Load test your LLM stack

Define load profiles. Measure TTFT, TPS, p50/p95/p99, errors.

## Quick Start

```bash
npx @stockyard/mcp-loadforge
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-loadforge": {
      "command": "npx",
      "args": ["@stockyard/mcp-loadforge"],
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
| `loadforge_setup` | Download and start the LoadForge proxy |
| `loadforge_stats` | Get load test results. |
| `loadforge_configure_client` | Get client configuration instructions |

## Part of Stockyard

LoadForge is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
