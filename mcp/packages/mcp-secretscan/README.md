# @stockyard/mcp-secretscan

> Catch API keys leaking through LLM calls

**Detect and redact API keys, AWS credentials, tokens, and secrets in LLM requests and responses via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-secretscan": {
      "command": "npx",
      "args": ["@stockyard/mcp-secretscan"],
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
    "stockyard-secretscan": {
      "command": "npx",
      "args": ["@stockyard/mcp-secretscan"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-secretscan": {
      "command": "npx",
      "args": ["@stockyard/mcp-secretscan"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up SecretScan"** — Downloads and starts the proxy
- **"Get scan statistics: total scanned, detections by type, blocks/redactions"**
- **"Test a text string for secret patterns without sending to LLM"**
- **"List active secret detection patterns"**
- **"Change what happens when a secret is detected"**
- **"List recent secret detections"**
- **"Check if the SecretScan proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `secretscan` binary for your platform
2. It writes a config and starts the proxy on port 5620
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5620/v1` to route through SecretScan
5. Dashboard available at `http://127.0.0.1:5620/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why SecretScan?

Detect and redact API keys, AWS credentials, tokens, and secrets in LLM requests and responses. TruffleHog-style pattern matching.

## Part of Stockyard

SecretScan is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
