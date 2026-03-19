# 📝 @stockyard/mcp-diffprompt

**DiffPrompt** — Git-style diff for prompt changes

Track system prompt changes. Hash-based detection. See which models had prompt modifications.

## Quick Start

```bash
npx @stockyard/mcp-diffprompt
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-diffprompt": {
      "command": "npx",
      "args": ["@stockyard/mcp-diffprompt"],
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
| `diffprompt_setup` | Download and start the DiffPrompt proxy |
| `diffprompt_stats` | Get change detection stats: prompts checked, changes detected. |
| `diffprompt_configure_client` | Get client configuration instructions |

## Part of Stockyard

DiffPrompt is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
