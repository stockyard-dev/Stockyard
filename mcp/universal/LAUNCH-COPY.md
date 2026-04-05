# MCP Directory Submissions & Launch Copy

## awesome-mcp PR

Add to the appropriate section (e.g. "Developer Tools" or "Infrastructure"):

```markdown
- [stockyard-mcp](https://github.com/stockyard-dev/Stockyard) — 384 MCP tools from 125 self-hosted developer tools. Feature flags, cron, webhooks, alerts, caching, cost tracking, secrets, and more. Single Go binary.
```

---

## Glama Submission

URL: https://glama.ai/mcp/servers/submit
- Repo: https://github.com/stockyard-dev/Stockyard
- Use `mcp/universal/glama.json` for listing details

## Smithery Submission

URL: https://smithery.ai/submit
- Repo: https://github.com/stockyard-dev/Stockyard
- Use `mcp/universal/smithery.yaml` for listing details

## mcp.so Submission

URL: https://mcp.so/submit
- Use `mcp/universal/mcp-so.json` for listing details

---

## Reddit Posts

### r/cursor

Title: I built 384 MCP tools for Cursor — feature flags, cron, alerts, caching, and 120 more

Body:
yall i've been building a collection of self-hosted developer tools for the past year. each one is a single Go binary with an embedded SQLite database. feature flags, cron scheduling, webhook relay, alerting, expense tracking, secrets management, 125 tools total.

the problem was always context switching. you're in Cursor writing code, then you tab over to check a feature flag, then back to Cursor, then over to the alert dashboard.

so i built stockyard-mcp. one binary that connects Cursor to every running tool via MCP. 384 MCP tools total. you type "is the dark-mode flag enabled?" and it calls the feature flag API. "what's my LLM spend this month?" calls the cost tracker. "schedule a backup at 2am" calls the cron scheduler.

setup is just adding this to .cursor/mcp.json:

```json
{
  "mcpServers": {
    "stockyard": {
      "command": "stockyard-mcp",
      "args": ["--scan", "4000-9900"]
    }
  }
}
```

it auto-discovers running tools on the port range and exposes them all.

7MB binary, zero deps, works with any Stockyard tool. also works with Claude Desktop, Windsurf, and Cline.

docs: https://stockyard.dev/mcp/
source: https://github.com/stockyard-dev/Stockyard

what tools do you wish you could talk to from your editor?

### r/ClaudeAI

Title: 384 MCP tools for Claude Desktop — self-hosted feature flags, cron, alerts, and more

(Same body, adjust "Cursor" references to "Claude Desktop" and the config example to claude_desktop_config.json)

### r/selfhosted

Title: I built 125 self-hosted developer tools with MCP support — talk to them from your AI editor

(Same body, emphasize self-hosted angle, no cloud, no third-party data sharing)

### r/LocalLLaMA

Title: 384 MCP tools for your local AI setup — works with any MCP client

(Same body, emphasize local-first, works with local models via Stockyard proxy)

---

## Show HN

Title: Show HN: 384 MCP tools for AI editors – one binary connects Cursor/Claude to 125 self-hosted tools

Body:
I've been building Stockyard (https://stockyard.dev) — a collection of 125 self-hosted developer tools. Each ships as a single Go binary (~12MB) with embedded SQLite and zero external dependencies.

Today I'm shipping stockyard-mcp, a universal MCP adapter that connects any MCP-compatible AI editor to every running Stockyard tool. 384 MCP tools from one 7MB binary.

The idea: instead of context-switching between your editor and various tool dashboards, just ask your editor. "Is the dark-mode flag enabled?" calls the feature flag API. "What's my LLM spend?" calls the cost tracker.

Architecture is simple: the adapter embeds a catalog of all tool API schemas, connects to running instances via HTTP, and serves MCP over stdio. JSON-RPC in, HTTP out.

Works with Cursor, Claude Desktop, Windsurf, and Cline. Setup is one JSON block in your editor config.

Blog post: https://stockyard.dev/blog/384-mcp-tools/
Source: https://github.com/stockyard-dev/Stockyard

---

## MCP Community Posts

### Cursor Community / Discord

Title: 384 MCP tools — feature flags, cron, webhooks, alerts, and more

Just shipped stockyard-mcp. One binary that connects Cursor to 125 self-hosted developer tools via MCP.

Add to .cursor/mcp.json:
```json
{"mcpServers":{"stockyard":{"command":"stockyard-mcp","args":["--scan","4000-9900"]}}}
```

It auto-discovers running tools and exposes 384 MCP endpoints. Feature flags, cron scheduling, webhook relay, alerting, cost tracking, caching, secrets, uptime monitoring, and more.

https://stockyard.dev/mcp/
