# 🧠 @stockyard/mcp-semanticcache

**SemanticCache** — Cache hits for similar prompts, not just identical

Embed prompts. Cosine similarity. Configurable threshold. 10x hit rate.

## Quick Start

```bash
npx @stockyard/mcp-semanticcache
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-semanticcache": {
      "command": "npx",
      "args": ["@stockyard/mcp-semanticcache"],
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
| `semanticcache_setup` | Download and start the SemanticCache proxy |
| `semanticcache_stats` | Get semantic cache stats. |
| `semanticcache_configure_client` | Get client configuration instructions |

## Part of Stockyard

SemanticCache is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
