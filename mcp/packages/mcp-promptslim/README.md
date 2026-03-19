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

PromptSlim is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
