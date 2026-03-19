# PromptLint

**Catch prompt anti-patterns before they cost you.**

PromptLint performs static analysis on prompts: detects redundancy, conflicting instructions, injection vulnerabilities, and missing format specs.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptlint

# Your app:   http://localhost:5840/v1/chat/completions
# Dashboard:  http://localhost:5840/ui
```

## What You Get

- Redundancy detection
- Conflict detection
- Injection vulnerability scanning
- Missing format spec warnings
- Prompt quality scoring
- CLI and middleware modes

## Config

```yaml
# promptlint.yaml
port: 5840
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
promptlint:
  mode: warn  # warn | block
  checks:
    - redundancy
    - conflicts
    - injection_risk
    - missing_format
  min_score: 0.5
```

## Docker

```bash
docker run -p 5840:5840 -e OPENAI_API_KEY=sk-... stockyard/promptlint
```

## Part of Stockyard

PromptLint is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
