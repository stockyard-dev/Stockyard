# @stockyard/mcp-toxicfilter

> Content moderation for LLM outputs

**Content moderation middleware for LLM responses via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-toxicfilter": {
      "command": "npx",
      "args": ["@stockyard/mcp-toxicfilter"],
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
    "stockyard-toxicfilter": {
      "command": "npx",
      "args": ["@stockyard/mcp-toxicfilter"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-toxicfilter": {
      "command": "npx",
      "args": ["@stockyard/mcp-toxicfilter"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up ToxicFilter"** — Downloads and starts the proxy
- **"Get moderation statistics: total scanned, blocks, flags, breakdown by category"**
- **"Test a text string against moderation rules without sending to LLM"**
- **"Change the default moderation action"**
- **"List active moderation categories and their rules"**
- **"Check if the ToxicFilter proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `toxicfilter` binary for your platform
2. It writes a config and starts the proxy on port 5600
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5600/v1` to route through ToxicFilter
5. Dashboard available at `http://127.0.0.1:5600/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why ToxicFilter?

Content moderation middleware for LLM responses. Block, redact, or flag harmful, hateful, or unsafe content before it reaches users.

## Part of Stockyard

ToxicFilter is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
