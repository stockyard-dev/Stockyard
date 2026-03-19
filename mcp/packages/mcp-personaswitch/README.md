# 🎭 @stockyard/mcp-personaswitch

**PersonaSwitch** — Hot-swap AI personalities without code changes

Define personas. Route by header/key/segment. Each: prompt, temperature, rules.

## Quick Start

```bash
npx @stockyard/mcp-personaswitch
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-personaswitch": {
      "command": "npx",
      "args": ["@stockyard/mcp-personaswitch"],
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
| `personaswitch_setup` | Download and start the PersonaSwitch proxy |
| `personaswitch_stats` | Get persona stats. |
| `personaswitch_personas` | List personas. |
| `personaswitch_configure_client` | Get client configuration instructions |

## Part of Stockyard

PersonaSwitch is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
