# 🐛 @stockyard/mcp-promptfuzz

**PromptFuzz** — Fuzz-test your prompts

Generate adversarial, multilingual, edge-case inputs. Score with EvalGate. Report failures.

## Quick Start

```bash
npx @stockyard/mcp-promptfuzz
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-promptfuzz": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptfuzz"],
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
| `promptfuzz_setup` | Download and start the PromptFuzz proxy |
| `promptfuzz_stats` | Get fuzz test stats. |
| `promptfuzz_configure_client` | Get client configuration instructions |

## Part of Stockyard

PromptFuzz is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
