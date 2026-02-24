# StructuredShield

**Your LLM responses always match your schema. Always.**

StructuredShield validates LLM responses against JSON schemas and automatically retries when validation fails. 96%+ pass rate with zero code changes.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/jsonguard

# Dashboard → http://localhost:4102/ui
```

## What You Get

- **JSON Schema validation** on every response
- **Auto-retry** (up to 3x) with the validation error in the prompt
- **Pass rate dashboard** — see your success rate in real-time
- **Per-schema stats** — track which schemas cause issues
- **Failure browser** — expected vs actual JSON, side by side

Part of [Stockyard](https://github.com/stockyard-dev/stockyard).
