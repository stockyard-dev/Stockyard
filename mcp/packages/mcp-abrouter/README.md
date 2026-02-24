# 🧪 @stockyard/mcp-abrouter

**ABRouter** — A/B test any LLM variable with statistical rigor

Run experiments across models, prompts, temperatures. Weighted traffic splits with automatic significance testing.

## Quick Start

```bash
npx @stockyard/mcp-abrouter
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-abrouter": {
      "command": "npx",
      "args": ["@stockyard/mcp-abrouter"],
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
| `abrouter_setup` | Download and start the ABRouter proxy |
| `abrouter_experiments` | List active experiments with variant stats. |
| `abrouter_create` | Create a new A/B experiment. |
| `abrouter_results` | Get statistical results for an experiment. |
| `abrouter_configure_client` | Get client configuration instructions |

## Part of Stockyard

ABRouter is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
