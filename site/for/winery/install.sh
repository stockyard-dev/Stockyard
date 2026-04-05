#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Wineries & Vineyards      │"
echo "  │  7 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/winery/       │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Harvest..."
  if curl -fsSL "https://stockyard.dev/harvest/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Harvest"
  else
    echo "    ✗ Harvest (failed — try manually: curl stockyard.dev/harvest/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Recipe..."
  if curl -fsSL "https://stockyard.dev/recipe/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Recipe"
  else
    echo "    ✗ Recipe (failed — try manually: curl stockyard.dev/recipe/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Menu..."
  if curl -fsSL "https://stockyard.dev/menu/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Menu"
  else
    echo "    ✗ Menu (failed — try manually: curl stockyard.dev/menu/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Quartermaster..."
  if curl -fsSL "https://stockyard.dev/quartermaster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Quartermaster"
  else
    echo "    ✗ Quartermaster (failed — try manually: curl stockyard.dev/quartermaster/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Dossier..."
  if curl -fsSL "https://stockyard.dev/dossier/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dossier"
  else
    echo "    ✗ Dossier (failed — try manually: curl stockyard.dev/dossier/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Steward..."
  if curl -fsSL "https://stockyard.dev/steward/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Steward"
  else
    echo "    ✗ Steward (failed — try manually: curl stockyard.dev/steward/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Booking..."
  if curl -fsSL "https://stockyard.dev/booking/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Booking"
  else
    echo "    ✗ Booking (failed — try manually: curl stockyard.dev/booking/install.sh | sh)"
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
