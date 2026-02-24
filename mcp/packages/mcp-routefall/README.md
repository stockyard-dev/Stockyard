# @stockyard/mcp-routefall

> LLM calls that never fail

**Automatic failover between LLM providers via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-routefall": {
      "command": "npx",
      "args": ["@stockyard/mcp-routefall"],
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
    "stockyard-routefall": {
      "command": "npx",
      "args": ["@stockyard/mcp-routefall"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-routefall": {
      "command": "npx",
      "args": ["@stockyard/mcp-routefall"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up FallbackRouter"** — Downloads and starts the proxy
- **"Provider status"** — Which providers are up/down
- **"Routing stats"** — Failover counts, circuit breaker trips
- **"Change primary"** — Switch the default provider
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `routefall` binary for your platform
2. It writes a config and starts the proxy on port 4400
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:4400/v1` to route through FallbackRouter
5. Dashboard available at `http://127.0.0.1:4400/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GROQ_API_KEY`)

## Why FallbackRouter?

Every LLM API call you make is unmonitored, uncached, and unprotected. FallbackRouter sits between your app and the API provider as an invisible proxy layer, adding automatic failover and circuit breaking with zero code changes.

## Part of Stockyard

FallbackRouter is one of 20 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all 20 tools for \$19/mo (saves 89% vs buying individually).

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
