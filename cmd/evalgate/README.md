# EvalGate

**Score every response. Retry the bad ones.**

EvalGate runs quality validators on LLM responses and auto-retries when quality is below threshold. Validators include JSON parsing, length checks, regex matching, and custom expressions.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/evalgate

# Your app:   http://localhost:4730/v1/chat/completions
# Dashboard:  http://localhost:4730/ui
```

## What You Get

- Response quality scoring
- Auto-retry on low quality
- JSON parse validation
- Min/max length checks
- Regex pattern matching
- Configurable retry budget

## Config

```yaml
# evalgate.yaml
port: 4730
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
eval:
  validators:
    - type: json_parse
    - type: min_length
      value: 50
    - type: regex_match
      pattern: "\\b(answer|result)\\b"
  retry:
    max_attempts: 3
    min_score: 0.7
```

## Docker

```bash
docker run -p 4730:4730 -e OPENAI_API_KEY=sk-... stockyard/evalgate
```

## Part of Stockyard

EvalGate is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use EvalGate standalone.
