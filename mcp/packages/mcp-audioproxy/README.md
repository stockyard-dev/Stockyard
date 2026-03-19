# 🔊 @stockyard/mcp-audioproxy

**AudioProxy** — Proxy for speech-to-text and text-to-speech

Cache TTS, track per-minute costs, failover between STT/TTS providers.

## Quick Start

```bash
npx @stockyard/mcp-audioproxy
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-audioproxy": {
      "command": "npx",
      "args": ["@stockyard/mcp-audioproxy"],
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
| `audioproxy_setup` | Download and start the AudioProxy proxy |
| `audioproxy_stats` | Get audio proxy stats. |
| `audioproxy_configure_client` | Get client configuration instructions |

## Part of Stockyard

AudioProxy is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
