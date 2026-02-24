# @stockyard/mcp-tracelink

> Distributed tracing for LLM chains

**Link related LLM calls into trace trees via MCP.**

## Quick Start

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-tracelink": {
      "command": "npx",
      "args": ["@stockyard/mcp-tracelink"],
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
    "stockyard-tracelink": {
      "command": "npx",
      "args": ["@stockyard/mcp-tracelink"]
    }
  }
}
```

### Windsurf / Cline / Claude Code

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "stockyard-tracelink": {
      "command": "npx",
      "args": ["@stockyard/mcp-tracelink"]
    }
  }
}
```

## Available Tools

Once connected, ask your AI assistant:

- **"Set up TraceLink"** — Downloads and starts the proxy
- **"List recent traces with root span info, total duration, and span count"**
- **"Get full trace tree with all spans for a trace ID"**
- **"Get tracing statistics: total traces, avg spans per trace, avg duration"**
- **"Search traces by model, duration, or span count"**
- **"Check if the TraceLink proxy is running and healthy"**
- **"How do I configure my app?"** — Get setup instructions for OpenAI SDK, LangChain, curl, etc.

## How It Works

1. The MCP server downloads the Stockyard `tracelink` binary for your platform
2. It writes a config and starts the proxy on port 5630
3. MCP tools communicate with the proxy's management REST API
4. Point your LLM client at `http://127.0.0.1:5630/v1` to route through TraceLink
5. Dashboard available at `http://127.0.0.1:5630/ui`

## Requirements

- Node.js 18+
- An LLM API key (set `OPENAI_API_KEY`)

## Why TraceLink?

Link related LLM calls into trace trees. Correlate multi-step agent workflows. OpenTelemetry-compatible trace propagation with waterfall visualization.

## Part of Stockyard

TraceLink is one of 32 Stockyard products. Get the full suite at [stockyard.dev](https://stockyard.dev) — all tools for \$19/mo (saves 89% vs buying individually).

## License

MIT
