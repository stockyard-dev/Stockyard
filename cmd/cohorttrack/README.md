# CohortTrack

**User cohort analytics for LLM products.**

CohortTrack groups users into cohorts by signup date, plan, or feature and tracks retention, cost, and engagement per cohort.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/cohorttrack

# Your app:   http://localhost:6450/v1/chat/completions
# Dashboard:  http://localhost:6450/ui
```

## What You Get

- Cohort grouping (signup, plan, feature)
- Retention tracking
- Cost per cohort
- Engagement metrics
- BI export
- Dashboard with cohort charts

## Config

```yaml
# cohorttrack.yaml
port: 6450
cohorttrack:
  dimensions: [signup_month, plan, feature]
  retention_windows: [7d, 30d, 90d]
```

## Docker

```bash
docker run -p 6450:6450 -e OPENAI_API_KEY=sk-... stockyard/cohorttrack
```

## Part of Stockyard

CohortTrack is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
