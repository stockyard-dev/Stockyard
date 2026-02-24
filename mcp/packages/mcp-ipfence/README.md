# @stockyard/mcp-ipfence

> IP allowlisting for your LLM endpoints

**IP-level access control for LLM proxy endpoints via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-ipfence": {
      "command": "npx",
      "args": ["@stockyard/mcp-ipfence"],
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
    "stockyard-ipfence": {
      "command": "npx",
      "args": ["@stockyard/mcp-ipfence"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-ipfence": {
      "command": "npx",
      "args": ["@stockyard/mcp-ipfence"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up IPFence"** — Downloads and starts the proxy
- **"Get access control statistics: requests checked, blocked, allowed, unique IPs"**
- **"Add an IP or CIDR range to the allowlist"**
- **"Add an IP or CIDR range to the denylist"**
- **"Check if a specific IP would be allowed or blocked"**
- **"List recent access events (allowed and blocked)"**
- **"Check if the IPFence proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `ipfence` binary for your platform
2. It writes a config and starts the proxy on port 5690
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5690/v1` to route through IPFence
5. Dashboard available at `http://127.0.0.1:5690/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why IPFence?

IP-level access control for LLM proxy endpoints. Allowlist, denylist, CIDR ranges. Block unauthorized access before any request processing.

## Part of Stockyard

IPFence is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
