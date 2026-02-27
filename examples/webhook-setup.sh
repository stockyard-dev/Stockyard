#!/bin/bash
# Set up Stockyard webhook alerting

BASE="http://localhost:4200"

echo "=== Register Slack webhook ==="
curl -s -X POST "$BASE/api/webhooks" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
    "secret": "my-signing-secret",
    "events": "alert.fired,cost.threshold,trust.violation"
  }' | jq .

echo -e "\n=== Register generic HTTP webhook ==="
curl -s -X POST "$BASE/api/webhooks" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/api/stockyard-events",
    "secret": "hmac-secret-123",
    "events": "*"
  }' | jq .

echo -e "\n=== List webhooks ==="
curl -s "$BASE/api/webhooks" | jq .

echo -e "\n=== Send test event ==="
curl -s -X POST "$BASE/api/webhooks/test" | jq .

echo -e "\n=== Set up a cost alert ==="
curl -s -X POST "$BASE/api/observe/alerts" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Daily cost spike",
    "metric": "cost",
    "threshold": 10.0,
    "window": "24h",
    "action": "alert"
  }' | jq .
