# ProfitCalc — Expected Profit Calculator

Estimate your expected profit when running Stockyard as an LLM proxy for your application or SaaS platform.

## Quick Start

```bash
# Run with built-in example scenarios
go run ./cmd/profitcalc

# Run with a custom scenario file
go run ./cmd/profitcalc scenario.json

# List all supported models and their pricing
go run ./cmd/profitcalc --models
```

## How It Works

ProfitCalc models the economics of running Stockyard as your LLM gateway:

- **Revenue**: Provider cost passthrough + your markup percentage
- **Costs**: LLM provider API costs + Stockyard license + infrastructure
- **Profit**: Revenue minus all costs, with per-request unit economics

The calculator uses the same embedded pricing table as Stockyard's spend tracking (47 models across 11 providers), so estimates match what you'll see in production.

## Scenario JSON

```json
{
  "name": "My SaaS",
  "tier": "pro",
  "monthly_requests": 100000,
  "avg_input_tokens": 800,
  "avg_output_tokens": 500,
  "model_mix": {
    "gpt-4o": 0.5,
    "claude-sonnet-4-5-20250929": 0.3,
    "gemini-2.0-flash": 0.2
  },
  "markup_pct": 25,
  "cache_hit_rate": 0.15,
  "infra_monthly_usd": 20
}
```

You can also pass an array of scenarios for comparison.

## Fields

| Field | Description |
|---|---|
| `tier` | `community`, `individual`, `pro`, `team`, `enterprise` |
| `monthly_requests` | Expected requests/month (capped by tier limits) |
| `avg_input_tokens` | Average input tokens per request |
| `avg_output_tokens` | Average output tokens per request |
| `model_mix` | Model → traffic share (values should sum to 1.0) |
| `markup_pct` | % markup on provider costs you charge customers |
| `cache_hit_rate` | Fraction of cache hits (reduces provider costs) |
| `infra_monthly_usd` | Monthly hosting/infrastructure costs |
| `team_seats` | Extra seats beyond 5 included (Team tier, $25/seat) |
