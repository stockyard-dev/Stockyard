# @stockyard/mcp-chatmem

> Persistent conversation memory without token bloat

**Conversation memory middleware via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-chatmem": {
      "command": "npx",
      "args": ["@stockyard/mcp-chatmem"],
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
    "stockyard-chatmem": {
      "command": "npx",
      "args": ["@stockyard/mcp-chatmem"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-chatmem": {
      "command": "npx",
      "args": ["@stockyard/mcp-chatmem"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up ChatMem"** — Downloads and starts the proxy
- **"List active conversation sessions with message counts and last activity"**
- **"Get memory state for a specific session"**
- **"Clear memory for a specific session"**
- **"Get memory statistics: active sessions, total messages stored, token savings"**
- **"Change memory management strategy"**
- **"Check if the ChatMem proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `chatmem` binary for your platform
2. It writes a config and starts the proxy on port 5650
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5650/v1` to route through ChatMem
5. Dashboard available at `http://127.0.0.1:5650/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why ChatMem?

Conversation memory middleware. Sliding window, summarization, and importance-based strategies. Persist memory across sessions without eating context windows.

## Part of Stockyard

ChatMem is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
