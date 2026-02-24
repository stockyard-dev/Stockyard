# 🏋️ @stockyard/mcp-llmbench

**LLMBench** — Benchmark any model on YOUR workload

Per-model performance tracking: latency, cost, tokens. Compare models on your actual traffic.

## Quick Start

```bash
npx @stockyard/mcp-llmbench
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-llmbench": {
      "command": "npx",
      "args": ["@stockyard/mcp-llmbench"],
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
| `llmbench_setup` | Download and start the LLMBench proxy |
| `llmbench_stats` | Get benchmark results per model. |
| `llmbench_compare` | Compare two models side by side. |
| `llmbench_configure_client` | Get client configuration instructions |

## Part of Stockyard

LLMBench is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
