# DocParse

**Preprocess documents for LLM context.**

DocParse extracts text from PDFs, Word docs, and HTML. Smart chunking and artifact cleaning before documents hit the LLM.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/docparse

# Your app:   http://localhost:6170/v1/chat/completions
# Dashboard:  http://localhost:6170/ui
```

## What You Get

- PDF text extraction
- Word doc parsing
- HTML cleaning
- Smart chunking strategies
- Artifact removal
- REST API for uploads

## Config

```yaml
# docparse.yaml
port: 6170
docparse:
  chunk_size: 1000
  chunk_overlap: 200
  clean_artifacts: true
  supported: [pdf, docx, html, txt, md]
```

## Docker

```bash
docker run -p 6170:6170 -e OPENAI_API_KEY=sk-... stockyard/docparse
```

## Part of Stockyard

DocParse is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use DocParse standalone.
