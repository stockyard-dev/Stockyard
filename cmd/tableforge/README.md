# TableForge

**LLM-powered table generation with validation.**

TableForge validates tabular/CSV output from LLMs. Checks columns, data types, completeness, and auto-repairs.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tableforge

# Your app:   http://localhost:6090/v1/chat/completions
# Dashboard:  http://localhost:6090/ui
```

## What You Get

- Table output detection
- Column validation
- Data type checking
- Completeness scoring
- Auto-repair malformed rows
- CSV/JSON export

## Config

```yaml
# tableforge.yaml
port: 6090
tableforge:
  expected_columns: [name, email, role]
  types: { name: string, email: email, role: string }
  require_complete: true
```

## Docker

```bash
docker run -p 6090:6090 -e OPENAI_API_KEY=sk-... stockyard/tableforge
```

## Part of Stockyard

TableForge is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TableForge standalone.
