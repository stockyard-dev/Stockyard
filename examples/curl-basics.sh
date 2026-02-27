#!/bin/bash
# Stockyard curl examples — all the key endpoints

BASE="http://localhost:4200"

echo "=== Health ==="
curl -s "$BASE/health" | jq .

echo -e "\n=== Chat completion ==="
curl -s "$BASE/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }' | jq .choices[0].message.content

echo -e "\n=== List modules ==="
curl -s "$BASE/api/proxy/modules" | jq '.modules | length'

echo -e "\n=== Toggle a module ==="
curl -s -X PUT "$BASE/api/proxy/modules/costcap" \
  -d '{"enabled": true}' | jq .

echo -e "\n=== Recent traces ==="
curl -s "$BASE/api/observe/traces?limit=3" | jq '.traces[] | {model, provider, latency_ms, cost}'

echo -e "\n=== Cost breakdown ==="
curl -s "$BASE/api/observe/costs" | jq .

echo -e "\n=== System status ==="
curl -s "$BASE/api/status" | jq '{status, uptime, total_requests, avg_latency_ms}'

echo -e "\n=== Trust ledger ==="
curl -s "$BASE/api/trust/ledger?limit=3" | jq '.events[] | {type, timestamp}'

echo -e "\n=== Exchange packs ==="
curl -s "$BASE/api/exchange/packs" | jq '.packs[] | {slug, name}'

echo -e "\n=== Apps ==="
curl -s "$BASE/api/apps" | jq .
