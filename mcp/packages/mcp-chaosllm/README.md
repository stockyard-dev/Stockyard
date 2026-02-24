# 💥 @stockyard/mcp-chaosllm

**ChaosLLM** — Chaos engineering for your LLM stack

Inject realistic failures: 429s, timeouts, malformed JSON, truncated streams.

## Quick Start

```bash
npx @stockyard/mcp-chaosllm
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-chaosllm": {
      "command": "npx",
      "args": ["@stockyard/mcp-chaosllm"],
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
| `chaosllm_setup` | Download and start the ChaosLLM proxy |
| `chaosllm_stats` | Get chaos injection stats. |
| `chaosllm_configure_client` | Get client configuration instructions |

## Part of Stockyard

ChaosLLM is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
