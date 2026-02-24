# 🔗 @stockyard/mcp-webhookrelay

**WebhookRelay** — Trigger LLM calls from any webhook

Receive webhooks, extract data, build prompts, call LLM, send results. GitHub→summarize→Slack in one config.

## Quick Start

```bash
npx @stockyard/mcp-webhookrelay
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-webhookrelay": {
      "command": "npx",
      "args": ["@stockyard/mcp-webhookrelay"],
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
| `webhookrelay_setup` | Download and start the WebhookRelay proxy |
| `webhookrelay_stats` | Get relay stats: webhooks received, calls triggered. |
| `webhookrelay_triggers` | List configured webhook triggers. |
| `webhookrelay_configure_client` | Get client configuration instructions |

## Part of Stockyard

WebhookRelay is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
