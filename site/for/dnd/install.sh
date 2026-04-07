#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for D&D & TTRPG Groups        │"
echo "  │  5 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/dnd/          │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Notebook..."
  if curl -fsSL "https://stockyard.dev/notebook/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Notebook"
  else
    echo "    ✗ Notebook (failed — try manually: curl stockyard.dev/notebook/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Booking..."
  if curl -fsSL "https://stockyard.dev/booking/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Booking"
  else
    echo "    ✗ Booking (failed — try manually: curl stockyard.dev/booking/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Roster..."
  if curl -fsSL "https://stockyard.dev/roster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Roster"
  else
    echo "    ✗ Roster (failed — try manually: curl stockyard.dev/roster/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Dossier..."
  if curl -fsSL "https://stockyard.dev/dossier/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dossier"
  else
    echo "    ✗ Dossier (failed — try manually: curl stockyard.dev/dossier/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sundial..."
  if curl -fsSL "https://stockyard.dev/sundial/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sundial"
  else
    echo "    ✗ Sundial (failed — try manually: curl stockyard.dev/sundial/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 5 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
