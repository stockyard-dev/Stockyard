# 🪞 @stockyard/mcp-mirrortest

**MirrorTest** — Shadow test new models against production traffic

Send production traffic to a shadow model. Compare quality, latency, cost. Zero user impact.

## Quick Start

```bash
npx @stockyard/mcp-mirrortest
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-mirrortest": {
      "command": "npx",
      "args": ["@stockyard/mcp-mirrortest"],
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
| `mirrortest_setup` | Download and start the MirrorTest proxy |
| `mirrortest_stats` | Get shadow test stats: requests mirrored, success rates. |
| `mirrortest_configure_client` | Get client configuration instructions |

## Part of Stockyard

MirrorTest is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
