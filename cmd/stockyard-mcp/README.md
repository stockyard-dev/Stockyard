# stockyard-mcp

Universal MCP adapter for [Stockyard](https://stockyard.dev) tools. One binary connects your AI editor to every running Stockyard tool — feature flags, cron jobs, alerts, caching, cost tracking, and 120+ more.

**384 MCP tools. One binary. Zero dependencies.**

## Quick Start

```bash
# Install
curl -fsSL https://stockyard.dev/stockyard-mcp/install.sh | sh

# Connect to specific tools
stockyard-mcp --tools costcap:4100,llmcache:4200

# Auto-discover tools on a port range
stockyard-mcp --scan 4000-7000

# Connect to all known tools at default ports
stockyard-mcp --all
```

## Editor Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "stockyard": {
      "command": "stockyard-mcp",
      "args": ["--tools", "costcap:4100,llmcache:4200"]
    }
  }
}
```

### Cursor

Create `.cursor/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "stockyard": {
      "command": "stockyard-mcp",
      "args": ["--tools", "costcap:4100,llmcache:4200"]
    }
  }
}
```

### Windsurf

Add to your Windsurf MCP config:

```json
{
  "mcpServers": {
    "stockyard": {
      "command": "stockyard-mcp",
      "args": ["--scan", "4000-7000"]
    }
  }
}
```

### Cline (VS Code)

In Cline settings, add:

```json
{
  "mcpServers": {
    "stockyard": {
      "command": "stockyard-mcp",
      "args": ["--all"]
    }
  }
}
```

## How It Works

```
Your AI Editor (Cursor/Claude/Windsurf)
    ↓ MCP protocol (stdio)
stockyard-mcp
    ↓ HTTP API calls
Running Stockyard tools (costcap:4100, llmcache:4200, ...)
```

The adapter:

1. Connects to running Stockyard tool instances via their HTTP APIs
2. Auto-generates MCP tool definitions from its embedded catalog
3. Translates MCP tool calls into HTTP requests to the correct tool
4. Returns results back through the MCP protocol

## Examples

Once connected, you can ask your AI editor things like:

- "What's my LLM spend this month?" → calls costcap
- "Show me the cache hit rate" → calls llmcache
- "Create a feature flag called dark-mode at 10% rollout" → calls saltlick
- "List all alert rules" → calls alertpulse
- "What's the circuit breaker status?" → calls retrypilot

## CLI Reference

```
stockyard-mcp [flags]

Flags:
  --tools string    Comma-separated product:port pairs
                    (e.g. costcap:4100,llmcache:4200)
  --scan string     Scan port range for tools (e.g. 4000-7000)
  --host string     Host to connect to (default "127.0.0.1")
  --all             Register all products at default ports
  --list            List all known products and exit
  --version         Print version and exit
```

## All 125 Products

Run `stockyard-mcp --list` to see every product with its default port, or browse them at [stockyard.dev/tools](https://stockyard.dev/tools/).

## Building from Source

```bash
git clone https://github.com/stockyard-dev/Stockyard.git
cd Stockyard
go build -o stockyard-mcp ./cmd/stockyard-mcp/
```

## License

BSL 1.1 — see [LICENSE](../../LICENSE) for details.
