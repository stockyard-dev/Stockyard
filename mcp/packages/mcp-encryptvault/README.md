# 🔐 @stockyard/mcp-encryptvault

**EncryptVault** — End-to-end encryption for sensitive LLM payloads

AES-GCM encryption for sensitive fields. Customer-managed keys. HIPAA/SOC2 compliance ready.

## Quick Start

```bash
npx @stockyard/mcp-encryptvault
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-encryptvault": {
      "command": "npx",
      "args": ["@stockyard/mcp-encryptvault"],
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
| `encryptvault_setup` | Download and start the EncryptVault proxy |
| `encryptvault_stats` | Get encryption stats: fields encrypted/decrypted. |
| `encryptvault_configure_client` | Get client configuration instructions |

## Part of Stockyard

EncryptVault is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
