# @stockyard/mcp-idlekill

> Kill runaway LLM requests before they drain your wallet

**Request watchdog middleware via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-idlekill": {
      "command": "npx",
      "args": ["@stockyard/mcp-idlekill"],
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
    "stockyard-idlekill": {
      "command": "npx",
      "args": ["@stockyard/mcp-idlekill"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-idlekill": {
      "command": "npx",
      "args": ["@stockyard/mcp-idlekill"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up IdleKill"** — Downloads and starts the proxy
- **"Get watchdog statistics: total monitored, killed, reasons for kills"**
- **"List currently active LLM requests being monitored"**
- **"Update kill thresholds"**
- **"List recently killed requests with reasons"**
- **"Check if the IdleKill proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `idlekill` binary for your platform
2. It writes a config and starts the proxy on port 5680
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5680/v1` to route through IdleKill
5. Dashboard available at `http://127.0.0.1:5680/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why IdleKill?

Request watchdog middleware. Kill LLM requests exceeding time, token, or cost limits. Stop agent loops, hanging streams, and runaway completions.

## Part of Stockyard

IdleKill is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
