# 🎙️ @stockyard/mcp-voicebridge

**VoiceBridge** — LLM middleware for voice/TTS pipelines

Strip markdown, URLs, code blocks from responses. Convert to speakable prose for voice assistants.

## Quick Start

```bash
npx @stockyard/mcp-voicebridge
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-voicebridge": {
      "command": "npx",
      "args": ["@stockyard/mcp-voicebridge"],
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
| `voicebridge_setup` | Download and start the VoiceBridge proxy |
| `voicebridge_stats` | Get voice optimization stats: elements stripped, avg length. |
| `voicebridge_configure_client` | Get client configuration instructions |

## Part of Stockyard

VoiceBridge is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
