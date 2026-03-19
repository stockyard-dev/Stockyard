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

ChainForge is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
