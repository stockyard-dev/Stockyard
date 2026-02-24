# ⏰ @stockyard/mcp-cronllm

**CronLLM** — Scheduled LLM tasks — your AI cron job runner

Define scheduled prompts in YAML. Daily summaries, weekly reports, periodic checks. Runs through full proxy chain.

## Quick Start

```bash
npx @stockyard/mcp-cronllm
```

## Add to Claude Desktop / Cursor

```json
{
  "mcpServers": {
    "stockyard-cronllm": {
      "command": "npx",
      "args": ["@stockyard/mcp-cronllm"],
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
| `cronllm_setup` | Download and start the CronLLM proxy |
| `cronllm_stats` | Get job execution stats. |
| `cronllm_jobs` | List scheduled jobs. |
| `cronllm_configure_client` | Get client configuration instructions |

## Part of Stockyard

CronLLM is one of 125 products in the [Stockyard](https://stockyard.dev) LLM infrastructure suite. Use standalone or as part of the full suite.

## License

MIT
