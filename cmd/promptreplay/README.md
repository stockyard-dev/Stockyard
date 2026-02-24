# PromptReplay

**See every LLM call your app made. Replay any of them.**

PromptReplay logs full request/response bodies and lets you replay any call — with the same model or a different one. Debug production issues, compare models, optimize prompts.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptreplay

# Dashboard → http://localhost:4105/ui
```

## What You Get

- **Full request/response logging** — every prompt, every response
- **Replay any request** — same model or try a different one
- **Model comparison** — replay with Claude vs GPT, see the diff
- **Search and filter** — find requests by model, time, cost
- **Export** — CSV and JSON export for analysis

Part of [Stockyard](https://github.com/stockyard-dev/stockyard).
