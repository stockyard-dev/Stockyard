#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Minecraft Server Admins  │"
echo "  │  9 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/minecraft/    │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Headcount..."
  if curl -fsSL "https://stockyard.dev/headcount/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Headcount"
  else
    echo "    ✗ Headcount (failed — try manually: curl stockyard.dev/headcount/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sentinel..."
  if curl -fsSL "https://stockyard.dev/sentinel/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sentinel"
  else
    echo "    ✗ Sentinel (failed — try manually: curl stockyard.dev/sentinel/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sundial..."
  if curl -fsSL "https://stockyard.dev/sundial/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sundial"
  else
    echo "    ✗ Sundial (failed — try manually: curl stockyard.dev/sundial/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Corral..."
  if curl -fsSL "https://stockyard.dev/corral/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Corral"
  else
    echo "    ✗ Corral (failed — try manually: curl stockyard.dev/corral/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Campfire..."
  if curl -fsSL "https://stockyard.dev/campfire/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Campfire"
  else
    echo "    ✗ Campfire (failed — try manually: curl stockyard.dev/campfire/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Announcements..."
  if curl -fsSL "https://stockyard.dev/announcements/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Announcements"
  else
    echo "    ✗ Announcements (failed — try manually: curl stockyard.dev/announcements/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Silo..."
  if curl -fsSL "https://stockyard.dev/silo/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Silo"
  else
    echo "    ✗ Silo (failed — try manually: curl stockyard.dev/silo/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Roster..."
  if curl -fsSL "https://stockyard.dev/roster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Roster"
  else
    echo "    ✗ Roster (failed — try manually: curl stockyard.dev/roster/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Outpost..."
  if curl -fsSL "https://stockyard.dev/outpost/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Outpost"
  else
    echo "    ✗ Outpost (failed — try manually: curl stockyard.dev/outpost/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "  ✓ All 9 tools installed!"
else
  echo ""
  echo "  Installed 9 tools ($FAILED had issues)"
fi

echo ""
echo "  Each tool runs on its own port with a web dashboard at /ui"
echo "  Free tier: 5 items per tool. Upgrade: stockyard.dev/pricing/?bundle=minecraft"
echo ""
echo "  Questions? hello@stockyard.dev"
echo ""
