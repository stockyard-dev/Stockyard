# WhiteLabel

**Your brand on Stockyard's engine.**

WhiteLabel replaces Stockyard branding with yours. Custom logos, colors, domain, and product name in the dashboard.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/whitelabel

# Your app:   http://localhost:5970/v1/chat/completions
# Dashboard:  http://localhost:5970/ui
```

## What You Get

- Custom logo upload
- Brand color configuration
- Product name override
- Custom domain support
- CSS customization
- Suite top-tier only

## Config

```yaml
# whitelabel.yaml
port: 5970
whitelabel:
  brand_name: "YourBrand AI"
  logo_url: "/assets/your-logo.svg"
  primary_color: "#3B82F6"
  accent_color: "#10B981"
  custom_domain: "ai.yourbrand.com"
```

## Docker

```bash
docker run -p 5970:5970 -e OPENAI_API_KEY=sk-... stockyard/whitelabel
```

## Part of Stockyard

WhiteLabel is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use WhiteLabel standalone.
