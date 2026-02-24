# Helicone + Stockyard

Stockyard and Helicone can coexist:
- Stockyard: infrastructure (caching, rate limiting, failover, cost caps)
- Helicone: logging and analytics

Option 1: Stockyard proxies to Helicone which proxies to OpenAI
  stockyard.yml providers.openai.base_url: "https://oai.hconeai.com/v1"

Option 2: Stockyard webhook sends logs to Helicone
  stockyard.yml webhooks.log_url: "https://api.helicone.ai/v1/log"

