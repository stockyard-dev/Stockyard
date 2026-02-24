# 📋 @stockyard/mcp-slotfill

**SlotFill** — Form-filling conversation engine

Declarative slot definitions. Track filled/missing. Reprompt. Completion funnels.

## Quick Start

```bash
npx @stockyard/mcp-slotfill
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-slotfill": {
      "command": "npx",
      "args": ["@stockyard/mcp-slotfill"],
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
| `slotfill_setup` | Download and start the SlotFill proxy |
| `slotfill_stats` | Get slot fill stats. |
| `slotfill_configure_client` | Get client configuration instructions |

## Part of Stockyard

SlotFill is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
