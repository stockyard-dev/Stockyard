# 📜 @stockyard/mcp-policyengine

**PolicyEngine** — Codify AI governance as enforceable rules

YAML policy rules compiled to middleware. Audit log. Compliance rate dashboard.

## Quick Start

```bash
npx @stockyard/mcp-policyengine
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-policyengine": {
      "command": "npx",
      "args": ["@stockyard/mcp-policyengine"],
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
| `policyengine_setup` | Download and start the PolicyEngine proxy |
| `policyengine_stats` | Get policy enforcement stats. |
| `policyengine_configure_client` | Get client configuration instructions |

## Part of Stockyard

PolicyEngine is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
