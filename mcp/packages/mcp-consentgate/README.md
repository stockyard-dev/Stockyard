# ✋ @stockyard/mcp-consentgate

**ConsentGate** — User consent management for AI interactions

Check consent per user. Block non-consented. Track timestamps. Support withdrawal.

## Quick Start

```bash
npx @stockyard/mcp-consentgate
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-consentgate": {
      "command": "npx",
      "args": ["@stockyard/mcp-consentgate"],
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
| `consentgate_setup` | Download and start the ConsentGate proxy |
| `consentgate_stats` | Get consent stats. |
| `consentgate_configure_client` | Get client configuration instructions |

## Part of Stockyard

ConsentGate is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
