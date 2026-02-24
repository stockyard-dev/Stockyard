# @stockyard/mcp-compliancelog

> Immutable audit trail for every LLM call

**Tamper-proof audit logging for LLM interactions via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-compliancelog": {
      "command": "npx",
      "args": ["@stockyard/mcp-compliancelog"],
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
    "stockyard-compliancelog": {
      "command": "npx",
      "args": ["@stockyard/mcp-compliancelog"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-compliancelog": {
      "command": "npx",
      "args": ["@stockyard/mcp-compliancelog"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up ComplianceLog"** — Downloads and starts the proxy
- **"Get audit log statistics: total entries, storage size, oldest/newest entry"**
- **"Search audit logs by date range, model, user, or project"**
- **"Verify hash chain integrity"**
- **"Export audit logs in compliance format"**
- **"Check if the ComplianceLog proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `compliancelog` binary for your platform
2. It writes a config and starts the proxy on port 5610
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5610/v1` to route through ComplianceLog
5. Dashboard available at `http://127.0.0.1:5610/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why ComplianceLog?

Tamper-proof audit logging for LLM interactions. Hash-chained entries, configurable retention, SOC2/HIPAA-ready export formats.

## Part of Stockyard

ComplianceLog is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
