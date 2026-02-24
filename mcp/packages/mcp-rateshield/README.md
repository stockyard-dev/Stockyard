# @stockyard/mcp-rateshield

> Bulletproof your LLM rate limits

**Rate limiting and request queuing for LLM APIs via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-rateshield": {
      "command": "npx",
      "args": ["@stockyard/mcp-rateshield"],
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
    "stockyard-rateshield": {
      "command": "npx",
      "args": ["@stockyard/mcp-rateshield"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-rateshield": {
      "command": "npx",
      "args": ["@stockyard/mcp-rateshield"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up RateShield"** — Downloads and starts the proxy
- **"Limit status"** — Remaining requests and tokens
- **"Update limits"** — Change rate limits dynamically
- **"Queue stats"** — Queued, processing, rejected requests
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `rateshield` binary for your platform
2. It writes a config and starts the proxy on port 4500
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:4500/v1` to route through RateShield
5. Dashboard available at `http://127.0.0.1:4500/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GROQ_API_KEY`)

## Why RateShield?

Every LLM API call you make is unmonitored, uncached, and unprotected. RateShield sits between your app and the API provider as an invisible proxy layer, adding rate limiting and request queuing with zero code changes.

## Part of Stockyard

RateShield is one of 20 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all 20 tools for \$19/mo (saves 89% vs buying individually).

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
