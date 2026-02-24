# ChainForge

**Multi-step LLM workflows as YAML.**

ChainForge defines multi-step LLM pipelines in YAML. Chain extract, analyze, summarize, and format steps with conditional branching.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/chainforge

# Your app:   http://localhost:5930/v1/chat/completions
# Dashboard:  http://localhost:5930/ui
```

## What You Get

- YAML pipeline definitions
- Data passing between steps
- Conditional branching
- Parallel execution
- Per-pipeline cost tracking
- Dashboard with pipeline monitor

## Config

```yaml
# chainforge.yaml
port: 5930
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
pipelines:
  summarize_and_translate:
    steps:
      - name: summarize
        model: gpt-4o
        prompt: "Summarize: {{input}}"
      - name: translate
        model: gpt-4o-mini
        prompt: "Translate to Spanish: {{summarize.output}}"
```

## Docker

```bash
docker run -p 5930:5930 -e OPENAI_API_KEY=sk-... stockyard/chainforge
```

## Part of Stockyard

ChainForge is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ChainForge standalone.
