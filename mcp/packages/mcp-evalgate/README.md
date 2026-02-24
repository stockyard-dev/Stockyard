# @stockyard/mcp-evalgate

> Only ship quality LLM responses

Response quality scoring and auto-retry. Validate JSON, check length, match regex, run custom expressions. Auto-retry on failure.

## Quick Start

```bash
npx @stockyard/mcp-evalgate
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-evalgate": {
      "command": "npx",
      "args": ["@stockyard/mcp-evalgate"],
      "env": {
        "OPENAI_API_KEY": "your-key-here"
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
    "stockyard-evalgate": {
      "command": "npx",
      "args": ["@stockyard/mcp-evalgate"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `evalgate_stats` | Get evaluation statistics: pass/fail rate, retry counts, validator breakdown |
| `evalgate_validators` | List active validators and their pass rates |
| `evalgate_add_validator` | Add a new response validator |
| `evalgate_test` | Test a response string against all active validators |
| `evalgate_proxy_status` | Check if the EvalGate proxy is running and healthy |

## How It Works

1. On first run, downloads the `evalgate` binary for your platform
2. Starts the EvalGate proxy on port 4110
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4110/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4110/ui` for the real-time EvalGate dashboard.

## Part of Stockyard

EvalGate is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
