#!/bin/bash
# Stockyard integration smoke test
# Builds the binary, starts it, hits every major endpoint, and verifies responses.
# Usage: ./test/smoke_test.sh
# Exit code: 0 on success, 1 on any failure

set -euo pipefail

BINARY="./dist/stockyard-smoke"
PORT=14200
BASE="http://localhost:${PORT}"
PID=""
PASS=0
FAIL=0
DATA_DIR=$(mktemp -d)

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$DATA_DIR" "$BINARY"
}
trap cleanup EXIT

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() { ((PASS++)); printf "  ${GREEN}✓${NC} %s\n" "$1"; }
fail() { ((FAIL++)); printf "  ${RED}✗${NC} %s: %s\n" "$1" "$2"; }

check() {
  local name="$1"
  local method="$2"
  local path="$3"
  local expect_code="${4:-200}"
  local body="${5:-}"

  local args=(-s -o /tmp/smoke-body -w '%{http_code}' -X "$method")
  if [ -n "$body" ]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi

  local code
  code=$(curl "${args[@]}" "${BASE}${path}" 2>/dev/null) || code="000"

  if [ "$code" = "$expect_code" ]; then
    pass "$name (HTTP $code)"
  else
    fail "$name" "expected $expect_code, got $code — $(cat /tmp/smoke-body 2>/dev/null | head -c 200)"
  fi
}

check_json() {
  local name="$1"
  local path="$2"
  local jq_filter="$3"
  local expect="$4"

  local result
  result=$(curl -s "${BASE}${path}" 2>/dev/null | jq -r "$jq_filter" 2>/dev/null) || result=""

  if [ "$result" = "$expect" ]; then
    pass "$name"
  else
    fail "$name" "expected '$expect', got '$result'"
  fi
}

echo "=== Stockyard Smoke Test ==="
echo ""

# 1. Build
echo "Building..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BINARY" ./cmd/stockyard 2>&1
pass "Binary built"

# 2. Start
echo "Starting on port $PORT..."
PORT=$PORT DATA_DIR="$DATA_DIR" STOCKYARD_ADMIN_KEY=smoke-test-key "$BINARY" &
PID=$!
disown "$PID" 2>/dev/null || true

# 3. Wait for health
echo "Waiting for health..."
READY=false
for i in $(seq 1 30); do
  if curl -s "${BASE}/health" >/dev/null 2>&1; then
    READY=true
    break
  fi
  sleep 0.5
done

if [ "$READY" = true ]; then
  pass "Server healthy (${i}s)"
else
  fail "Server start" "not healthy after 15s"
  echo "FATAL: Server did not start. Aborting."
  exit 1
fi

echo ""
echo "--- Core ---"
check "GET /health" GET /health
check_json "Health status" /health ".status" "ok"
check "GET /api/status" GET /api/status
check_json "Status healthy" /api/status ".status" "healthy"
check "GET /api/apps" GET /api/apps
check "GET /api/plans" GET /api/plans
check "GET /api/license" GET /api/license
check "GET /api/openapi.json" GET /api/openapi.json

echo ""
echo "--- Proxy ---"
check "GET /api/proxy/modules" GET /api/proxy/modules
check_json "Modules exist" /api/proxy/modules ".modules | length > 0" "true"
check "GET /api/proxy/providers" GET /api/proxy/providers

echo ""
echo "--- Observe ---"
check "GET /api/observe/overview" GET /api/observe/overview
check "GET /api/observe/traces" GET /api/observe/traces
check "GET /api/observe/timeseries" GET "/api/observe/timeseries?period=24h"
check "GET /api/observe/costs" GET /api/observe/costs

echo ""
echo "--- Trust ---"
check "GET /api/trust/ledger" GET /api/trust/ledger
check "GET /api/trust/policies" GET /api/trust/policies

echo ""
echo "--- Studio ---"
check "GET /api/studio/status" GET /api/studio/status
check "GET /api/studio/templates" GET /api/studio/templates

echo ""
echo "--- Forge ---"
check "GET /api/forge/workflows" GET /api/forge/workflows

echo ""
echo "--- Exchange ---"
check "GET /api/exchange/packs" GET /api/exchange/packs

echo ""
echo "--- Auth ---"
check "POST /api/auth/signup" POST /api/auth/signup 200 '{"email":"smoke@test.dev","password":"test1234"}'

echo ""
echo "--- Webhooks ---"
check "POST /api/webhooks" POST /api/webhooks 200 '{"url":"https://example.com/smoke","events":"*"}'
check "GET /api/webhooks" GET /api/webhooks
check_json "Webhook created" /api/webhooks ".webhooks | length" "1"
check "DELETE /api/webhooks/1" DELETE /api/webhooks/1

echo ""
echo "--- Playground Share ---"
# Create a share
SHARE_RESP=$(curl -s -X POST "${BASE}/api/playground/share" \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"Hello"}],"model":"gpt-4o-mini"}')
SHARE_ID=$(echo "$SHARE_RESP" | jq -r '.id' 2>/dev/null)
if [ -n "$SHARE_ID" ] && [ "$SHARE_ID" != "null" ]; then
  pass "Create playground share (id=$SHARE_ID)"
  check "GET playground share" GET "/api/playground/share/$SHARE_ID"
  check_json "Share model" "/api/playground/share/$SHARE_ID" ".model" "gpt-4o-mini"
else
  fail "Create playground share" "$SHARE_RESP"
fi

echo ""
echo "--- Config ---"
check "GET /api/config/export" GET /api/config/export
check_json "Export has modules" /api/config/export ".modules | length > 0" "true"

echo ""
echo "--- Site Pages ---"
check "GET / (homepage)" GET /
check "GET /playground" GET /playground
check "GET /docs/" GET /docs/
check "GET /pricing/" GET /pricing/
check "GET /blog/" GET /blog/
check "GET /status/" GET /status/
check "GET /architecture/" GET /architecture/
check "GET /benchmarks/" GET /benchmarks/
check "GET /changelog/" GET /changelog/
check "GET /vs/litellm/" GET /vs/litellm/
check "GET /docs/ops/" GET /docs/ops/
check "GET /blog/feed.xml" GET /blog/feed.xml
check "GET /sitemap.xml" GET /sitemap.xml
check "GET /robots.txt" GET /robots.txt
check "GET /404 (branded)" GET /nonexistent 404

echo ""
echo "=== Results ==="
echo "  ${PASS} passed, ${FAIL} failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "SMOKE TEST FAILED"
  exit 1
else
  echo "ALL SMOKE TESTS PASSED"
  exit 0
fi
