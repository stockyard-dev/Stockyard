#!/bin/bash
# Stockyard Pre-Launch Sanity Check
# Run this Monday night before scheduling the PH listing.
# Everything should be green. Fix any reds before launch.

set -e
HOST="https://stockyard.dev"
PASS=0
FAIL=0
WARN=0

green() { printf "\033[0;32m  ✅ %s\033[0m\n" "$1"; PASS=$((PASS+1)); }
red()   { printf "\033[0;31m  ❌ %s\033[0m\n" "$1"; FAIL=$((FAIL+1)); }
warn()  { printf "\033[0;33m  ⚠️  %s\033[0m\n" "$1"; WARN=$((WARN+1)); }

check_url() {
  code=$(curl -s -o /dev/null -w '%{http_code}' "$1")
  if [ "$code" = "200" ]; then green "$2"; else red "$2 → HTTP $code"; fi
}

echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║  STOCKYARD PRE-LAUNCH SANITY CHECK              ║"
echo "╚══════════════════════════════════════════════════╝"
echo ""

echo "── SITE ──"
check_url "$HOST/" "Homepage"
check_url "$HOST/pricing/" "Pricing"
check_url "$HOST/guide/" "Guide"
check_url "$HOST/playground/" "Playground"
check_url "$HOST/blog/" "Blog index"
check_url "$HOST/blog/66-modules-400ns/" "Blog: benchmarks"
check_url "$HOST/blog/llm-api-pricing-2026/" "Blog: pricing"
check_url "$HOST/about/" "About"
check_url "$HOST/docs/" "Docs"
check_url "$HOST/docs/quickstart/" "Quickstart"
check_url "$HOST/benchmarks/" "Benchmarks"
check_url "$HOST/architecture/" "Architecture"

echo ""
echo "── COMPARISON PAGES ──"
check_url "$HOST/vs/litellm/" "vs LiteLLM"
check_url "$HOST/vs/helicone/" "vs Helicone"
check_url "$HOST/vs/portkey/" "vs Portkey"

echo ""
echo "── AUDIENCE PAGES ──"
for p in startups enterprise developers gpu-owners experts builders; do
  check_url "$HOST/for/$p/" "for/$p"
done

echo ""
echo "── API ──"
check_url "$HOST/health" "Health"
check_url "$HOST/v1/models" "Models endpoint"
check_url "$HOST/api/plans" "Plans"
check_url "$HOST/api/apps" "Apps"

echo ""
echo "── CHECKOUT ──"
for plan in pro team enterprise; do
  for interval in monthly annual; do
    result=$(curl -s -X POST "$HOST/api/checkout" -H "Content-Type: application/json" \
      -d "{\"plan\":\"$plan\",\"interval\":\"$interval\",\"email\":\"prelaunch@test.com\"}")
    has_url=$(echo "$result" | grep -c '"url"')
    if [ "$has_url" -gt 0 ]; then green "Checkout: $plan/$interval"; else red "Checkout: $plan/$interval"; fi
  done
done

echo ""
echo "── INSTALL ──"
check_url "$HOST/install.sh" "Install script"
dl=$(curl -sI "https://github.com/stockyard-dev/Stockyard/releases/download/v1.0.0/stockyard_1.0.0_linux_amd64.tar.gz" | grep -i 'HTTP/' | tail -1 | grep -c '302\|200')
if [ "$dl" -gt 0 ]; then green "Binary download (linux/amd64)"; else red "Binary download failed"; fi

echo ""
echo "── PERFORMANCE ──"
ttfb=$(curl -s -o /dev/null -w '%{time_starttransfer}' -H "Accept-Encoding: gzip" "$HOST/")
if (( $(echo "$ttfb < 1.0" | bc -l) )); then green "TTFB: ${ttfb}s"; else warn "TTFB: ${ttfb}s (slow)"; fi

size=$(curl -s -o /dev/null -w '%{size_download}' -H "Accept-Encoding: gzip" "$HOST/")
if [ "$size" -lt 15000 ]; then green "Gzip size: ${size}B"; else warn "Gzip size: ${size}B (large)"; fi

gzip=$(curl -sI -H "Accept-Encoding: gzip" "$HOST/" | grep -c 'content-encoding: gzip')
if [ "$gzip" -gt 0 ]; then green "Gzip enabled"; else red "Gzip NOT enabled"; fi

cache=$(curl -sI "$HOST/site-assets/assets/screenshots/console-overview.webp" | grep -c 'immutable')
if [ "$cache" -gt 0 ]; then green "Asset caching (immutable)"; else warn "Asset caching missing"; fi

echo ""
echo "── SOCIAL ──"
og=$(curl -s "$HOST/" | grep -c 'og:image')
if [ "$og" -gt 0 ]; then green "OG image tag"; else red "OG image missing"; fi
check_url "$HOST/site-assets/assets/marketing/og-card.png" "OG card image"
check_url "$HOST/favicon.ico" "Favicon"

echo ""
echo "── SECURITY ──"
hsts=$(curl -sI "$HOST/" | grep -c 'strict-transport-security')
if [ "$hsts" -gt 0 ]; then green "HSTS header"; else warn "HSTS missing"; fi

csp=$(curl -sI "$HOST/" | grep -c 'content-security-policy')
if [ "$csp" -gt 0 ]; then green "CSP header"; else warn "CSP missing"; fi

echo ""
echo "── RSS / SEO ──"
check_url "$HOST/blog/feed.xml" "RSS feed"
check_url "$HOST/sitemap.xml" "Sitemap"
check_url "$HOST/robots.txt" "Robots.txt"

echo ""
echo "══════════════════════════════════════════════════"
printf "  \033[0;32m%d passed\033[0m  " "$PASS"
if [ "$FAIL" -gt 0 ]; then printf "\033[0;31m%d FAILED\033[0m  " "$FAIL"; fi
if [ "$WARN" -gt 0 ]; then printf "\033[0;33m%d warnings\033[0m" "$WARN"; fi
echo ""
echo "══════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "  ⛔ FIX THE FAILURES BEFORE LAUNCHING"
  exit 1
else
  echo ""
  echo "  🚀 READY TO LAUNCH"
fi
