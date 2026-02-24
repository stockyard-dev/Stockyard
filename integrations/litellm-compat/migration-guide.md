# Migrating from LiteLLM to Stockyard

## Why Migrate?
- LiteLLM: Python, needs Redis/Postgres, 35K+ stars but heavy
- Stockyard: Go single binary, SQLite only, no dependencies

## Step 1: Replace LiteLLM proxy with Stockyard
```bash
# Before: litellm --port 4000
# After:
npx @stockyard/mcp-stockyard
```

## Step 2: Same base URL, same API
Your code doesn't change. Stockyard speaks OpenAI-compatible API.

```python
# This works with both LiteLLM and Stockyard:
from openai import OpenAI
client = OpenAI(base_url="http://localhost:4000/v1", api_key="any")
```

## Step 3: Migrate config
```yaml
# stockyard.yml
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
```

## What You Gain
- 125 middleware products vs LiteLLM's proxy-only approach
- No Python runtime, no Redis, no Postgres
- 6MB static binary vs Python environment
- Embedded dashboard (no separate UI service)

