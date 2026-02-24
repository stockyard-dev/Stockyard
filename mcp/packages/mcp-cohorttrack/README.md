# 👥 @stockyard/mcp-cohorttrack

**CohortTrack** — User cohort analytics for LLM products

Cohorts by signup, plan, feature. Retention, cost per cohort. BI export.

## Quick Start

```bash
npx @stockyard/mcp-cohorttrack
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-cohorttrack": {
      "command": "npx",
      "args": ["@stockyard/mcp-cohorttrack"],
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
| `cohorttrack_setup` | Download and start the CohortTrack proxy |
| `cohorttrack_stats` | Get cohort analytics. |
| `cohorttrack_configure_client` | Get client configuration instructions |

## Part of Stockyard

CohortTrack is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
