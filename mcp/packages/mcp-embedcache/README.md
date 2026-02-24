# @stockyard/mcp-embedcache

> Never compute the same embedding twice

**Embedding response caching for /v1/embeddings via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-embedcache": {
      "command": "npx",
      "args": ["@stockyard/mcp-embedcache"],
      "env": {
        "OPENAI_API_KEY": "sk-your-key-here"
      }
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "stockyard-embedcache": {
      "command": "npx",
      "args": ["@stockyard/mcp-embedcache"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-embedcache": {
      "command": "npx",
      "args": ["@stockyard/mcp-embedcache"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up EmbedCache"** — Downloads and starts the proxy
- **"Get cache statistics: entries, hit rate, bytes saved, evictions"**
- **"Clear the embedding cache"**
- **"Check if a specific text has a cached embedding"**
- **"Change cache TTL for new entries"**
- **"Check if the EmbedCache proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `embedcache` binary for your platform
2. It writes a config and starts the proxy on port 5700
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5700/v1` to route through EmbedCache
5. Dashboard available at `http://127.0.0.1:5700/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why EmbedCache?

Embedding response caching for /v1/embeddings. Content-hash deduplication, per-input cache splitting, 7-day TTL. Slash embedding costs for RAG pipelines.

## Part of Stockyard

EmbedCache is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
