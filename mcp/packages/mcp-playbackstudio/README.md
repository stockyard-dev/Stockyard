# 🎪 @stockyard/mcp-playbackstudio

**PlaybackStudio** — Interactive playground for exploring logged interactions

Advanced filters. Conversation threads. Side-by-side. Bulk actions.

## Quick Start

```bash
npx @stockyard/mcp-playbackstudio
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-playbackstudio": {
      "command": "npx",
      "args": ["@stockyard/mcp-playbackstudio"],
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
| `playbackstudio_setup` | Download and start the PlaybackStudio proxy |
| `playbackstudio_stats` | Get exploration stats. |
| `playbackstudio_configure_client` | Get client configuration instructions |

## Part of Stockyard

PlaybackStudio is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
