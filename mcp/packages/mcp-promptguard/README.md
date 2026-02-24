# @stockyard/mcp-promptguard

> PII never hits the LLM

PII redaction and prompt injection detection for LLM APIs. Regex-based redaction with restore capability. Block or sanitize dangerous prompts.

## Quick Start

```bash
npx @stockyard/mcp-promptguard
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-promptguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptguard"],
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
    "stockyard-promptguard": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptguard"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `promptguard_stats` | Get guard statistics: total scanned, PII detections by type, injection blocks |
| `promptguard_test` | Test a prompt against PII detection and injection rules without sending to LLM |
| `promptguard_set_mode` | Change the guard mode: redact, redact-restore, or block |
| `promptguard_set_sensitivity` | Set injection detection sensitivity level |
| `promptguard_proxy_status` | Check if the PromptGuard proxy is running and healthy |

## How It Works

1. On first run, downloads the `promptguard` binary for your platform
2. Starts the PromptGuard proxy on port 4800
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4800/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4800/ui` for the real-time PromptGuard dashboard.

## Part of Stockyard

PromptGuard is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
