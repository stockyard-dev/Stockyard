# 📸 @stockyard/mcp-snapshottest

**SnapshotTest** — Snapshot testing for LLM outputs

Record baselines. Semantic diff. Configurable threshold. CI-friendly.

## Quick Start

```bash
npx @stockyard/mcp-snapshottest
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-snapshottest": {
      "command": "npx",
      "args": ["@stockyard/mcp-snapshottest"],
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
| `snapshottest_setup` | Download and start the SnapshotTest proxy |
| `snapshottest_stats` | Get snapshot test stats. |
| `snapshottest_configure_client` | Get client configuration instructions |

## Part of Stockyard

SnapshotTest is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
