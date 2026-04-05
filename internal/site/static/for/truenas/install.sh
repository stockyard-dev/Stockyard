#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for TrueNAS Users            │"
echo "  │  6 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/truenas/      │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Sentinel..."
  if curl -fsSL "https://stockyard.dev/sentinel/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sentinel"
  else
    echo "    ✗ Sentinel (failed — try manually: curl stockyard.dev/sentinel/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Outpost..."
  if curl -fsSL "https://stockyard.dev/outpost/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Outpost"
  else
    echo "    ✗ Outpost (failed — try manually: curl stockyard.dev/outpost/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sundial..."
  if curl -fsSL "https://stockyard.dev/sundial/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sundial"
  else
    echo "    ✗ Sundial (failed — try manually: curl stockyard.dev/sundial/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Tally..."
  if curl -fsSL "https://stockyard.dev/tally/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tally"
  else
    echo "    ✗ Tally (failed — try manually: curl stockyard.dev/tally/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Silo..."
  if curl -fsSL "https://stockyard.dev/silo/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Silo"
  else
    echo "    ✗ Silo (failed — try manually: curl stockyard.dev/silo/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Mainspring..."
  if curl -fsSL "https://stockyard.dev/mainspring/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Mainspring"
  else
    echo "    ✗ Mainspring (failed — try manually: curl stockyard.dev/mainspring/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "  ✓ All 6 tools installed!"
else
  echo ""
  echo "  Installed 6 tools ($FAILED had issues)"
fi

echo ""
echo "  Each tool runs on its own port with a web dashboard at /ui"
echo "  Free tier: 5 items per tool. Upgrade: stockyard.dev/pricing/?bundle=truenas"
echo ""
echo "  Questions? hello@stockyard.dev"
echo ""
