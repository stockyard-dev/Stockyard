# 🗃️ @stockyard/mcp-datamap

**DataMap** — GDPR Article 30 data flow mapping

Auto-classify data. Map flows: source→proxy→provider→storage. Generate GDPR records.

## Quick Start

```bash
npx @stockyard/mcp-datamap
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-datamap": {
      "command": "npx",
      "args": ["@stockyard/mcp-datamap"],
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
| `datamap_setup` | Download and start the DataMap proxy |
| `datamap_stats` | Get data mapping stats. |
| `datamap_configure_client` | Get client configuration instructions |

## Part of Stockyard

DataMap is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
