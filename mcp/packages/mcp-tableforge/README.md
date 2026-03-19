# 📊 @stockyard/mcp-tableforge

**TableForge** — LLM-powered CSV/table generation with validation

Detect tables in output. Validate columns, types, completeness. Auto-repair and export.

## Quick Start

```bash
npx @stockyard/mcp-tableforge
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-tableforge": {
      "command": "npx",
      "args": ["@stockyard/mcp-tableforge"],
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
| `tableforge_setup` | Download and start the TableForge proxy |
| `tableforge_stats` | Get table validation stats. |
| `tableforge_configure_client` | Get client configuration instructions |

## Part of Stockyard

TableForge is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
