# DiffPrompt

**Git-style diff for prompt changes.**

DiffPrompt compares two prompt versions against the same test inputs with side-by-side output diffs and quality scoring.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/diffprompt

# Your app:   http://localhost:6000/v1/chat/completions
# Dashboard:  http://localhost:6000/ui
```

## What You Get

- Side-by-side prompt comparison
- Shared test input sets
- Output diff visualization
- Quality scoring per version
- Cost comparison
- CLI and dashboard modes

## Config

```yaml
# diffprompt.yaml
port: 6000
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
diffprompt:
  test_inputs:
    - "What is machine learning?"
    - "Explain recursion."
    - "Write a haiku about coding."
```

## Docker

```bash
docker run -p 6000:6000 -e OPENAI_API_KEY=sk-... stockyard/diffprompt
```

## Part of Stockyard

DiffPrompt is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
