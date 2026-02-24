# 🎬 @stockyard/mcp-agentreplay

**AgentReplay** — Record and replay agent sessions step-by-step

Step-by-step playback on TraceLink data. What-if mode. Export as test cases.

## Quick Start

```bash
npx @stockyard/mcp-agentreplay
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-agentreplay": {
      "command": "npx",
      "args": ["@stockyard/mcp-agentreplay"],
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
| `agentreplay_setup` | Download and start the AgentReplay proxy |
| `agentreplay_stats` | Get replay stats. |
| `agentreplay_configure_client` | Get client configuration instructions |

## Part of Stockyard

AgentReplay is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
