# ToolShield

**Validate tool calls before execution.**

ToolShield intercepts LLM tool_use calls and validates arguments, enforces permissions, and rate limits per tool.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/toolshield

# Your app:   http://localhost:6110/v1/chat/completions
# Dashboard:  http://localhost:6110/ui
```

## What You Get

- Tool call argument validation
- Per-tool permissions
- Per-tool rate limits
- Block dangerous patterns
- Audit trail for tool calls
- Dashboard with tool call log

## Config

```yaml
# toolshield.yaml
port: 6110
toolshield:
  rules:
    delete_user: { blocked: true }
    send_email: { rate_limit: 10/hour }
    read_file: { allowed_paths: ["/safe/"] }
```

## Docker

```bash
docker run -p 6110:6110 -e OPENAI_API_KEY=sk-... stockyard/toolshield
```

## Part of Stockyard

ToolShield is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ToolShield standalone.
