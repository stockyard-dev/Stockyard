# @stockyard/mcp-tenantwall

> Per-tenant isolation for multi-tenant LLM apps

**Tenant isolation middleware via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-tenantwall": {
      "command": "npx",
      "args": ["@stockyard/mcp-tenantwall"],
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
    "stockyard-tenantwall": {
      "command": "npx",
      "args": ["@stockyard/mcp-tenantwall"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-tenantwall": {
      "command": "npx",
      "args": ["@stockyard/mcp-tenantwall"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up TenantWall"** — Downloads and starts the proxy
- **"List all known tenants with usage summary"**
- **"Get detailed usage for a specific tenant"**
- **"Set rate limits and spend caps for a tenant"**
- **"Block a tenant from making LLM requests"**
- **"Get multi-tenant statistics: active tenants, total spend, top consumers"**
- **"Check if the TenantWall proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `tenantwall` binary for your platform
2. It writes a config and starts the proxy on port 5670
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5670/v1` to route through TenantWall
5. Dashboard available at `http://127.0.0.1:5670/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why TenantWall?

Tenant isolation middleware. Per-tenant rate limits, spend caps, model access, and cache isolation. Build multi-tenant AI products without custom infrastructure.

## Part of Stockyard

TenantWall is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
