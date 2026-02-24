# ✂️ @stockyard/mcp-promptslim

**PromptSlim** — Compress prompts by 40-70% without losing meaning

Remove redundant whitespace, filler words, articles. Configurable aggressiveness. See before/after token savings.

## Quick Start

```bash
npx @stockyard/mcp-promptslim
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-promptslim": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptslim"],
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
| `promptslim_setup` | Download and start the PromptSlim proxy |
| `promptslim_stats` | Get compression stats: chars saved, tokens saved, compression ratio. |
| `promptslim_configure_client` | Get client configuration instructions |

## Part of Stockyard

PromptSlim is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
