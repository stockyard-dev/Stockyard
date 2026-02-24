# 💻 @stockyard/mcp-codelang

**CodeLang** — Language-aware code generation with syntax validation

Tree-sitter parsing. Syntax errors, undefined refs, suspicious patterns.

## Quick Start

```bash
npx @stockyard/mcp-codelang
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-codelang": {
      "command": "npx",
      "args": ["@stockyard/mcp-codelang"],
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
| `codelang_setup` | Download and start the CodeLang proxy |
| `codelang_stats` | Get code validation stats. |
| `codelang_configure_client` | Get client configuration instructions |

## Part of Stockyard

CodeLang is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
