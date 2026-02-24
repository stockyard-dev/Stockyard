# 🔒 @stockyard/mcp-codefence

**CodeFence** — Validate LLM-generated code before it runs

Scan LLM code output for dangerous patterns: shell injection, file access, crypto mining. Block or flag unsafe code.

## Quick Start

```bash
npx @stockyard/mcp-codefence
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-codefence": {
      "command": "npx",
      "args": ["@stockyard/mcp-codefence"],
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
| `codefence_setup` | Download and start the CodeFence proxy |
| `codefence_stats` | Get code validation stats: scanned, flagged, blocked. |
| `codefence_patterns` | List active forbidden patterns. |
| `codefence_add_pattern` | Add a custom forbidden code pattern. |
| `codefence_configure_client` | Get client configuration instructions |

## Part of Stockyard

CodeFence is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
