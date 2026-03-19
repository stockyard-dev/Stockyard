# 🏷️ @stockyard/mcp-modelalias

**ModelAlias** — Abstract away model names with aliases

Aliases: fast→gpt-4o-mini, smart→claude-sonnet. Change mapping, all apps update.

## Quick Start

```bash
npx @stockyard/mcp-modelalias
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-modelalias": {
      "command": "npx",
      "args": ["@stockyard/mcp-modelalias"],
      "env": {
        "OPENAI_API_KEY": "your-key"
      }
    }
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `modelalias_setup` | Download and start the ModelAlias proxy |
| `modelalias_stats` | Get alias resolution stats. |
| `modelalias_list` | List active aliases. |
| `modelalias_configure_client` | Get client configuration instructions |

## Part of Stockyard

ModelAlias is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
