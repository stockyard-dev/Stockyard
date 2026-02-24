# @stockyard/mcp-anthrofit

> Use Claude with OpenAI SDKs

**Deep Anthropic compatibility layer via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-anthrofit": {
      "command": "npx",
      "args": ["@stockyard/mcp-anthrofit"],
      "env": {
        "ANTHROPIC_API_KEY": "sk-ant-your-key-here"
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
    "stockyard-anthrofit": {
      "command": "npx",
      "args": ["@stockyard/mcp-anthrofit"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-anthrofit": {
      "command": "npx",
      "args": ["@stockyard/mcp-anthrofit"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up AnthroFit"** — Downloads and starts the proxy
- **"Get translation statistics: requests processed, system prompts fixed, tools translated, errors"**
- **"Test OpenAI→Anthropic translation on a request without sending"**
- **"Change system prompt handling mode"**
- **"Get current AnthroFit configuration"**
- **"Check if the AnthroFit proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `anthrofit` binary for your platform
2. It writes a config and starts the proxy on port 5710
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5710/v1` to route through AnthroFit
5. Dashboard available at `http://127.0.0.1:5710/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `ANTHROPIC_API_KEY`)

## Why AnthroFit?

Deep Anthropic compatibility layer. System prompt consolidation, max_tokens injection, tool schema translation, streaming normalization. Drop-in Claude support for OpenAI apps.

## Part of Stockyard

AnthroFit is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
