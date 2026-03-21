# Zapier Integration for Stockyard

## Triggers
- **New Trace** — fires when a new LLM request is traced (polling)
- **Cost Alert** — fires when a cost threshold is exceeded (webhook)
- **Guardrail Violation** — fires when a guardrail rule is violated (webhook)

## Actions
- **Send Prompt** — send a chat completion through Stockyard
- **Toggle Module** — enable or disable a proxy module

## Files
- `triggers.json` — Zapier trigger definitions
- `actions.json` — Zapier action definitions

## Authentication
Set these in your Zapier connection:
- `base_url` — Your Stockyard instance URL (e.g., `https://your-instance.com`)
- `admin_key` — Admin API key

## Learn More
- [Stockyard Docs](https://stockyard.dev/docs/)
- [GitHub](https://github.com/stockyard-dev/stockyard)
