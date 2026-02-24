# SemanticCache

**Cache hits for similar prompts.**

SemanticCache embeds prompts and uses cosine similarity to match. 'Weather in NYC' and 'weather in New York City' become cache hits.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/semanticcache

# Your app:   http://localhost:6220/v1/chat/completions
# Dashboard:  http://localhost:6220/ui
```

## What You Get

- Embedding-based prompt similarity
- Cosine similarity matching
- Configurable similarity threshold
- 10x cache hit rate vs exact match
- EmbedCache integration
- Dashboard with similarity scores

## Config

```yaml
# semanticcache.yaml
port: 6220
semanticcache:
  threshold: 0.92
  embed_model: text-embedding-3-small
  max_entries: 50000
```

## Docker

```bash
docker run -p 6220:6220 -e OPENAI_API_KEY=sk-... stockyard/semanticcache
```

## Part of Stockyard

SemanticCache is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SemanticCache standalone.
