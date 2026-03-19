# 🎬 @stockyard/mcp-framegrab

**FrameGrab** — Extract and analyze video frames through vision LLMs

Scene detection. Batch frames. Smart frame selection. Cost per frame.

## Quick Start

```bash
npx @stockyard/mcp-framegrab
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-framegrab": {
      "command": "npx",
      "args": ["@stockyard/mcp-framegrab"],
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
| `framegrab_setup` | Download and start the FrameGrab proxy |
| `framegrab_stats` | Get frame extraction stats. |
| `framegrab_configure_client` | Get client configuration instructions |

## Part of Stockyard

FrameGrab is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
