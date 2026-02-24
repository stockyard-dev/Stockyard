# Migrating from Cloudflare AI Gateway to Stockyard

## Why?
- Cloudflare: limited to caching + rate limiting
- Stockyard: 125 middleware products
- Self-hosted: your data stays local

## Step 1: Replace gateway URL
Before: https://gateway.ai.cloudflare.com/v1/{account}/{gateway}
After:  http://your-stockyard:4000/v1

## Step 2: Move provider keys to Stockyard config
Your application code doesn't change.
