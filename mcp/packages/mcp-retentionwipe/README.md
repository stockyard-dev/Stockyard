# 🧹 @stockyard/mcp-retentionwipe

**RetentionWipe** — Automated data retention and deletion

Retention periods per data type. Auto-purge. Per-user deletion. Deletion certificates.

## Quick Start

```bash
npx @stockyard/mcp-retentionwipe
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-retentionwipe": {
      "command": "npx",
      "args": ["@stockyard/mcp-retentionwipe"],
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
| `retentionwipe_setup` | Download and start the RetentionWipe proxy |
| `retentionwipe_stats` | Get retention stats. |
| `retentionwipe_configure_client` | Get client configuration instructions |

## Part of Stockyard

RetentionWipe is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
