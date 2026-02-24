# CodeFence

**Validate LLM-generated code before it runs.**

CodeFence detects code blocks in LLM output and runs safety checks: syntax validation, forbidden pattern detection, and complexity scoring.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/codefence

# Your app:   http://localhost:5730/v1/chat/completions
# Dashboard:  http://localhost:5730/ui
```

## What You Get

- Code block detection
- Syntax validation
- Forbidden pattern matching
- Complexity scoring
- Language-aware checks
- Block or warn modes

## Config

```yaml
# codefence.yaml
port: 5730
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
codefence:
  mode: warn  # block | warn
  forbidden_patterns:
    - "rm -rf"
    - "eval("
    - "exec("
    - "DROP TABLE"
  max_complexity: 50
```

## Docker

```bash
docker run -p 5730:5730 -e OPENAI_API_KEY=sk-... stockyard/codefence
```

## Part of Stockyard

CodeFence is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use CodeFence standalone.
