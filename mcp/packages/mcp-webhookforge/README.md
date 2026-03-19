# ⚒️ @stockyard/mcp-webhookforge

**WebhookForge** — Visual builder for webhook→LLM→action pipelines

Visual flow builder. Trigger→transform→LLM→condition→action. History.

## Quick Start

```bash
npx @stockyard/mcp-webhookforge
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-webhookforge": {
      "command": "npx",
      "args": ["@stockyard/mcp-webhookforge"],
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
| `webhookforge_setup` | Download and start the WebhookForge proxy |
| `webhookforge_stats` | Get pipeline stats. |
| `webhookforge_configure_client` | Get client configuration instructions |

## Part of Stockyard

WebhookForge is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
