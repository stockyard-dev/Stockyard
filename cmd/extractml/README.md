# ExtractML

**Force structured data from free-text responses.**

ExtractML auto-injects extraction calls when models return prose instead of structured data.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/extractml

# Your app:   http://localhost:6080/v1/chat/completions
# Dashboard:  http://localhost:6080/ui
```

## What You Get

- Auto-extract structured data from prose
- Define output schemas
- Retry with extraction prompt
- Cache extraction patterns
- Works with any model
- Dashboard with extraction stats

## Config

```yaml
# extractml.yaml
port: 6080
extractml:
  schema:
    type: object
    properties:
      name: { type: string }
      age: { type: integer }
```

## Docker

```bash
docker run -p 6080:6080 -e OPENAI_API_KEY=sk-... stockyard/extractml
```

## Part of Stockyard

ExtractML is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ExtractML standalone.
