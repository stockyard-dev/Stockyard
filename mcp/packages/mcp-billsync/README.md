# 💰 @stockyard/mcp-billsync

**BillSync** — Per-customer LLM invoices automatically

Track usage per tenant. Apply markup. Generate invoice data. Stripe-compatible usage records.

## Quick Start

```bash
npx @stockyard/mcp-billsync
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-billsync": {
      "command": "npx",
      "args": ["@stockyard/mcp-billsync"],
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
| `billsync_setup` | Download and start the BillSync proxy |
| `billsync_stats` | Get billing stats: tenants, revenue, markup. |
| `billsync_tenants` | List tenant billing summaries. |
| `billsync_configure_client` | Get client configuration instructions |

## Part of Stockyard

BillSync is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.

## License

MIT
