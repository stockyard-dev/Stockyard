# @stockyard/mcp-contextpack

> RAG without the vector database

Rule-based context injection from local files. Keyword-matched chunks injected into prompts automatically. No embeddings, no vector DB.

## Quick Start

```bash
npx @stockyard/mcp-contextpack
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-contextpack": {
      "command": "npx",
      "args": ["@stockyard/mcp-contextpack"],
      "env": {
        "OPENAI_API_KEY": "your-key-here"
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
    "stockyard-contextpack": {
      "command": "npx",
      "args": ["@stockyard/mcp-contextpack"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `contextpack_sources` | List configured context sources and their chunk counts |
| `contextpack_add_source` | Add a new context source |
| `contextpack_search` | Search context sources for relevant chunks |
| `contextpack_stats` | Get context injection statistics: total injections, avg chunks, token overhead |
| `contextpack_reindex` | Re-index all context sources after adding new files |
| `contextpack_proxy_status` | Check if the ContextPack proxy is running and healthy |

## How It Works

1. On first run, downloads the `contextpack` binary for your platform
2. Starts the ContextPack proxy on port 5400
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:5400/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:5400/ui` for the real-time ContextPack dashboard.

## Part of Stockyard

ContextPack is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
