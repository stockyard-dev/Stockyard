#!/bin/bash
# Stockyard Launch Day Dashboard
# Run: bash launch-dashboard.sh
# Refreshes every 60 seconds. Press Ctrl+C to stop.

HOST="https://stockyard.dev"
AUTH="Authorization: Bearer ${STOCKYARD_ADMIN_KEY:-sy_admin_2026_080201832}"

while true; do
  clear
  echo "╔══════════════════════════════════════════════════╗"
  echo "║  STOCKYARD LAUNCH DASHBOARD  $(date '+%H:%M:%S %Z')   ║"
  echo "╚══════════════════════════════════════════════════╝"
  echo ""

  # Health
  health=$(curl -s -o /dev/null -w '%{http_code}' "$HOST/health")
  if [ "$health" = "200" ]; then
    echo "  🟢 Site: ONLINE"
  else
    echo "  🔴 Site: DOWN ($health)"
  fi

  # Response time
  ttfb=$(curl -s -o /dev/null -w '%{time_starttransfer}' "$HOST/")
  echo "  ⏱  TTFB: ${ttfb}s"
  echo ""

  # Email captures
  gate=$(curl -s -H "$AUTH" "$HOST/api/exchange/gate/stats" 2>/dev/null)
  total_emails=$(echo "$gate" | python3 -c "import sys,json;print(json.load(sys.stdin).get('total_captures',0))" 2>/dev/null)
  unique_emails=$(echo "$gate" | python3 -c "import sys,json;print(json.load(sys.stdin).get('unique_emails',0))" 2>/dev/null)
  echo "  📧 Emails: $unique_emails unique ($total_emails total)"

  # Users
  users=$(curl -s -H "$AUTH" "$HOST/api/auth/users" 2>/dev/null)
  user_count=$(echo "$users" | python3 -c "import sys,json;d=json.load(sys.stdin);print(len(d) if isinstance(d,list) else len(d.get('users',[])))" 2>/dev/null)
  echo "  👤 Users: $user_count registered"

  # Traces
  observe=$(curl -s -H "$AUTH" "$HOST/api/observe/overview" 2>/dev/null)
  traces=$(echo "$observe" | python3 -c "import sys,json;print(json.load(sys.stdin).get('total_traces',0))" 2>/dev/null)
  cost=$(echo "$observe" | python3 -c "import sys,json;print(f\"\${json.load(sys.stdin).get('total_cost_usd',0):.4f}\")" 2>/dev/null)
  echo "  📊 Traces: $traces | Cost tracked: $cost"

  # Exchange
  packs=$(curl -s -H "$AUTH" "$HOST/api/exchange/packs" 2>/dev/null)
  pack_count=$(echo "$packs" | python3 -c "import sys,json;print(json.load(sys.stdin).get('count',0))" 2>/dev/null)
  echo "  📦 Packs: $pack_count available"

  echo ""
  echo "  ── Recent email signups ──"
  echo "$gate" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for e in d.get('recent',[])[:5]:
    email = e['email']
    ts = e['created_at'][11:16] if len(e['created_at'])>11 else '?'
    src = e.get('source','?')
    print(f'    {ts}  {email}  ({src})')
" 2>/dev/null

  echo ""
  echo "  ── Key URLs ──"
  echo "    PH:    https://www.producthunt.com/posts/stockyard"
  echo "    Site:  https://stockyard.dev"
  echo "    GitHub: https://github.com/stockyard-dev/Stockyard"
  echo ""
  echo "  Refreshing in 60s... (Ctrl+C to stop)"
  sleep 60
done
