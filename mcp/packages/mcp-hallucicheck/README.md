# 🔍 @stockyard/mcp-hallucicheck

**HalluciCheck** — Catch LLM hallucinations before your users do

Validate URLs, emails, and citations in LLM responses. Flag or retry when models invent non-existent references.

## Quick Start

```bash
npx @stockyard/mcp-hallucicheck
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-hallucicheck": {
      "command": "npx",
      "args": ["@stockyard/mcp-hallucicheck"],
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
| `hallucicheck_setup` | Download and start the HalluciCheck proxy |
| `hallucicheck_stats` | Get hallucination detection stats: checked, invalid URLs/emails found. |
| `hallucicheck_recent` | List recent hallucination detections with details. |
| `hallucicheck_configure_client` | Get client configuration instructions |

## Part of Stockyard

HalluciCheck is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
