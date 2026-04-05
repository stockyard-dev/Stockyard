#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Board Game Groups         │"
echo "  │  9 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/board-gamer/  │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Quartermaster..."
  if curl -fsSL "https://stockyard.dev/quartermaster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Quartermaster"
  else
    echo "    ✗ Quartermaster (failed — try manually: curl stockyard.dev/quartermaster/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Roster..."
  if curl -fsSL "https://stockyard.dev/roster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Roster"
  else
    echo "    ✗ Roster (failed — try manually: curl stockyard.dev/roster/install.sh | sh)"
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

  echo "  Installing Notebook..."
  if curl -fsSL "https://stockyard.dev/notebook/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Notebook"
  else
    echo "    ✗ Notebook (failed — try manually: curl stockyard.dev/notebook/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Agora..."
  if curl -fsSL "https://stockyard.dev/agora/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Agora"
  else
    echo "    ✗ Agora (failed — try manually: curl stockyard.dev/agora/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Collection..."
  if curl -fsSL "https://stockyard.dev/collection/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Collection"
  else
    echo "    ✗ Collection (failed — try manually: curl stockyard.dev/collection/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Checkout..."
  if curl -fsSL "https://stockyard.dev/checkout/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Checkout"
  else
    echo "    ✗ Checkout (failed — try manually: curl stockyard.dev/checkout/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Tournament..."
  if curl -fsSL "https://stockyard.dev/tournament/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tournament"
  else
    echo "    ✗ Tournament (failed — try manually: curl stockyard.dev/tournament/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 9 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
