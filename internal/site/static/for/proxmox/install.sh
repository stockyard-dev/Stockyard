#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Proxmox Users             │"
echo "  │  7 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/proxmox/      │"
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

  echo "  Installing Mainspring..."
  if curl -fsSL "https://stockyard.dev/mainspring/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Mainspring"
  else
    echo "    ✗ Mainspring (failed — try manually: curl stockyard.dev/mainspring/install.sh | sh)"
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

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 7 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
