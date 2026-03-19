# CodeLang

**Language-aware code validation.**

CodeLang uses tree-sitter parsing for actual syntax validation of LLM-generated code. Finds undefined references and suspicious patterns.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/codelang

# Your app:   http://localhost:6550/v1/chat/completions
# Dashboard:  http://localhost:6550/ui
```

## What You Get

- Tree-sitter based parsing
- Syntax error detection
- Undefined reference checking
- Language-specific rules
- Multiple language support
- Dashboard with code quality metrics

## Config

```yaml
# codelang.yaml
port: 6550
codelang:
  languages: [python, javascript, go, rust]
  checks: [syntax, undefined_refs, suspicious_patterns]
```

## Docker

```bash
docker run -p 6550:6550 -e OPENAI_API_KEY=sk-... stockyard/codelang
```

## Part of Stockyard

CodeLang is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
