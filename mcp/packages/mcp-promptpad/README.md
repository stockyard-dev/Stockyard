# @stockyard/mcp-promptpad

> Version control for your prompts

Prompt template versioning and A/B testing. Store, version, and test prompt templates. Track which variants perform best.

## Quick Start

```bash
npx @stockyard/mcp-promptpad
```

## MCP Configuration

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "stockyard-promptpad": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptpad"],
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
    "stockyard-promptpad": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptpad"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `promptpad_list` | List all prompt templates with version numbers and usage stats |
| `promptpad_get` | Get a specific prompt template by name and optional version |
| `promptpad_save` | Save or update a prompt template |
| `promptpad_render` | Render a template with variables and optionally send to LLM |
| `promptpad_ab_stats` | Get A/B test results for prompt variants |
| `promptpad_proxy_status` | Check if the PromptPad proxy is running and healthy |

## How It Works

1. On first run, downloads the `promptpad` binary for your platform
2. Starts the PromptPad proxy on port 4801
3. Exposes management tools via MCP protocol
4. Your LLM client connects at `http://127.0.0.1:4801/v1/chat/completions`

## Dashboard

Open `http://127.0.0.1:4801/ui` for the real-time PromptPad dashboard.

## Part of Stockyard

PromptPad is one of 20 tools in the [Stockyard](https://stockyard.dev) suite. Install the full suite:

```bash
npx @stockyard/mcp-stockyard
```

## License

MIT
