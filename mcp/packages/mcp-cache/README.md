# @stockyard/mcp-cache

> Stop paying twice for the same LLM response

**LLM response caching with configurable TTL via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-cache": {
      "command": "npx",
      "args": ["@stockyard/mcp-cache"],
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
    "stockyard-cache": {
      "command": "npx",
      "args": ["@stockyard/mcp-cache"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-cache": {
      "command": "npx",
      "args": ["@stockyard/mcp-cache"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up CacheLayer"** — Downloads and starts the proxy
- **"Show cache stats"** — Hit rate, entries, savings
- **"Flush the cache"** — Clear cached responses
- **"Set cache TTL"** — Change expiration time
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `llmcache` binary for your platform
2. It writes a config and starts the proxy on port 4200
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:4200/v1` to route through CacheLayer
5. Dashboard available at `http://127.0.0.1:4200/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GROQ_API_KEY`)

## Why CacheLayer?

Every LLM API call you make is unmonitored, uncached, and unprotected. CacheLayer sits between your app and the API provider as an invisible proxy layer, adding intelligent response caching with zero code changes.

## Part of Stockyard

CacheLayer is one of 20 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all 20 tools for \$19/mo (saves 89% vs buying individually).

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
