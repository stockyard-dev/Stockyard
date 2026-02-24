# BillSync

**Per-customer LLM invoicing.**

BillSync generates per-customer invoices from UsagePulse data. Configure markup, billing periods, and export to Stripe or CSV.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/billsync

# Your app:   http://localhost:5960/v1/chat/completions
# Dashboard:  http://localhost:5960/ui
```

## What You Get

- Per-customer invoice generation
- Configurable markup percentages
- Billing period management
- Stripe usage record export
- CSV/PDF invoice export
- Dashboard with billing overview

## Config

```yaml
# billsync.yaml
port: 5960
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
billing:
  markup: 1.5           # 50% markup
  period: monthly
  currency: usd
  stripe_key: ${STRIPE_KEY}
  invoice_format: pdf
```

## Docker

```bash
docker run -p 5960:5960 -e OPENAI_API_KEY=sk-... stockyard/billsync
```

## Part of Stockyard

BillSync is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use BillSync standalone.
