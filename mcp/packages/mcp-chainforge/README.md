# ⛓️ @stockyard/mcp-chainforge

**ChainForge** — Multi-step LLM workflows as YAML pipelines

Define extract→analyze→summarize→format pipelines. Conditional branching, parallel execution, cost tracking per pipeline.

## Quick Start

```bash
npx @stockyard/mcp-chainforge
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-chainforge": {
      "command": "npx",
      "args": ["@stockyard/mcp-chainforge"],
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
| `chainforge_setup` | Download and start the ChainForge proxy |
| `chainforge_stats` | Get pipeline execution stats. |
| `chainforge_pipelines` | List configured pipelines. |
| `chainforge_configure_client` | Get client configuration instructions |

## Part of Stockyard

ChainForge is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
