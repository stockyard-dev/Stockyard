# 🔎 @stockyard/mcp-promptlint

**PromptLint** — Catch prompt anti-patterns before they cost you money

Static analysis for prompts: detect redundancy, injection patterns, excessive length. Score and suggest improvements.

## Quick Start

```bash
npx @stockyard/mcp-promptlint
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-promptlint": {
      "command": "npx",
      "args": ["@stockyard/mcp-promptlint"],
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
| `promptlint_setup` | Download and start the PromptLint proxy |
| `promptlint_stats` | Get lint stats: issues found by severity, top patterns. |
| `promptlint_configure_client` | Get client configuration instructions |

## Part of Stockyard

PromptLint is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
