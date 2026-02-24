# @stockyard/mcp-mockllm

> Deterministic LLM responses for testing

**Mock LLM server with canned responses for CI/CD pipelines via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-mockllm": {
      "command": "npx",
      "args": ["@stockyard/mcp-mockllm"],
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
    "stockyard-mockllm": {
      "command": "npx",
      "args": ["@stockyard/mcp-mockllm"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-mockllm": {
      "command": "npx",
      "args": ["@stockyard/mcp-mockllm"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up MockLLM"** — Downloads and starts the proxy
- **"List all configured mock fixtures with match patterns"**
- **"Add a new mock fixture"**
- **"Remove a mock fixture by ID"**
- **"Get mock server stats: total requests, fixture match rate, error simulations"**
- **"Switch mock mode: fixture (canned), passthrough (forward to real), record (capture + replay)"**
- **"Check if the MockLLM proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `mockllm` binary for your platform
2. It writes a config and starts the proxy on port 5660
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5660/v1` to route through MockLLM
5. Dashboard available at `http://127.0.0.1:5660/ui`

## Requirements

- Node.js 18+
- No API key required (MockLLM provides canned responses)

## Why MockLLM?

Mock LLM server with canned responses for CI/CD pipelines. Define fixtures, simulate errors, control latency. Never hit real APIs in tests.

## Part of Stockyard

MockLLM is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
