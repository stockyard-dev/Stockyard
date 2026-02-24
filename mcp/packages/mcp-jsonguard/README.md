# @stockyard/mcp-jsonguard

> LLM responses that always parse

**JSON schema validation for LLM responses via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-jsonguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-jsonguard"],
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
    "stockyard-jsonguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-jsonguard"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-jsonguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-jsonguard"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up StructuredShield"** — Downloads and starts the proxy
- **"Show validation stats"** — Pass rate, retry rate
- **"Register a schema"** — Add JSON schema for auto-validation
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `jsonguard` binary for your platform
2. It writes a config and starts the proxy on port 4300
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:4300/v1` to route through StructuredShield
5. Dashboard available at `http://127.0.0.1:4300/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GROQ_API_KEY`)

## Why StructuredShield?

Every LLM API call you make is unmonitored, uncached, and unprotected. StructuredShield sits between your app and the API provider as an invisible proxy layer, adding JSON validation with auto-retry with zero code changes.

## Part of Stockyard

StructuredShield is one of 20 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all 20 tools for \$19/mo (saves 89% vs buying individually).

| Product | What it does |
|---------|-------------|
| **CostCap** | Spending caps & budget tracking |
| **CacheLayer** | Response caching |
| **StructuredShield** | JSON schema validation |
| **FallbackRouter** | Provider failover |
| **RateShield** | Rate limiting |
| **PromptReplay** | Request logging & replay |
| + 14 more | KeyPool, PromptGuard, ModelSwitch, EvalGate, UsagePulse... |

## License

MIT
