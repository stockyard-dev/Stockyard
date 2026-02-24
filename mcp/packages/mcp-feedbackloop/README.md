# 👍 @stockyard/mcp-feedbackloop

**FeedbackLoop** — Close the LLM improvement loop

Collect user ratings and feedback linked to specific LLM requests. Track quality trends over time.

## Quick Start

```bash
npx @stockyard/mcp-feedbackloop
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-feedbackloop": {
      "command": "npx",
      "args": ["@stockyard/mcp-feedbackloop"],
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
| `feedbackloop_setup` | Download and start the FeedbackLoop proxy |
| `feedbackloop_stats` | Get feedback stats: total ratings, average score, trends. |
| `feedbackloop_submit` | Submit feedback for a request. |
| `feedbackloop_recent` | List recent feedback entries. |
| `feedbackloop_configure_client` | Get client configuration instructions |

## Part of Stockyard

FeedbackLoop is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
