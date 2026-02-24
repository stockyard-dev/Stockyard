# 🚧 @stockyard/mcp-guardrail

**GuardRail** — Keep your LLM on-script

Topic fencing middleware. Define allowed/denied topics. Block off-topic responses with custom fallback messages.

## Quick Start

```bash
npx @stockyard/mcp-guardrail
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-guardrail": {
      "command": "npx",
      "args": ["@stockyard/mcp-guardrail"],
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
| `guardrail_setup` | Download and start the GuardRail proxy |
| `guardrail_stats` | Get topic enforcement stats: blocked, allowed, violations. |
| `guardrail_topics` | List allowed and denied topic patterns. |
| `guardrail_configure_client` | Get client configuration instructions |

## Part of Stockyard

GuardRail is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
