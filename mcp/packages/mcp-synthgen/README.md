# 🧬 @stockyard/mcp-synthgen

**SynthGen** — Generate synthetic training data through your proxy

Templates + seed examples → synthetic training data at scale. Quality-checked through EvalGate.

## Quick Start

```bash
npx @stockyard/mcp-synthgen
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-synthgen": {
      "command": "npx",
      "args": ["@stockyard/mcp-synthgen"],
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
| `synthgen_setup` | Download and start the SynthGen proxy |
| `synthgen_stats` | Get generation stats: samples generated, batches run. |
| `synthgen_configure_client` | Get client configuration instructions |

## Part of Stockyard

SynthGen is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
