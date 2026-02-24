# ContextPack

**Poor man's RAG. Inject context from anywhere.**

ContextPack injects context from files, SQLite databases, or URLs into LLM prompts. Lightweight RAG without a vector database.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/contextpack

# Your app:   http://localhost:5400/v1/chat/completions
# Dashboard:  http://localhost:5400/ui
```

## What You Get

- Inject context from files, SQLite, or URLs
- Automatic chunking and relevance scoring
- Template-based context injection
- Configurable context budget
- Source attribution in responses
- No vector database needed

## Config

```yaml
# contextpack.yaml
port: 5400
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
context:
  sources:
    - type: file
      path: ./docs/
      glob: "*.md"
    - type: url
      urls:
        - https://docs.example.com/api
  max_tokens: 2000
  strategy: relevance  # relevance | all | random
```

## Docker

```bash
docker run -p 5400:5400 -e OPENAI_API_KEY=sk-... stockyard/contextpack
```

## Part of Stockyard

ContextPack is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ContextPack standalone.
