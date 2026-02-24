# ✅ @stockyard/mcp-approvalgate

**ApprovalGate** — Require human approval for prompt changes

Approval workflow for prompt modifications. Track who approved what and when. Audit trail included.

## Quick Start

```bash
npx @stockyard/mcp-approvalgate
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-approvalgate": {
      "command": "npx",
      "args": ["@stockyard/mcp-approvalgate"],
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
| `approvalgate_setup` | Download and start the ApprovalGate proxy |
| `approvalgate_stats` | Get approval stats: pending, approved, rejected. |
| `approvalgate_pending` | List pending approval requests. |
| `approvalgate_configure_client` | Get client configuration instructions |

## Part of Stockyard

ApprovalGate is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
