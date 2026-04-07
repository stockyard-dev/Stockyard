#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌───────────────────────────────────────────────┐"
echo "  │  Stockyard for Fermenters & Kombucha Brewers  │"
echo "  │  4 tools · $7.99/mo · self-hosted             │"
echo "  │  https://stockyard.dev/for/fermentation/      │"
echo "  └───────────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Recipe..."
  if curl -fsSL "https://stockyard.dev/recipe/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Recipe"
  else
    echo "    ✗ Recipe (failed — try manually: curl stockyard.dev/recipe/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Quartermaster..."
  if curl -fsSL "https://stockyard.dev/quartermaster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Quartermaster"
  else
    echo "    ✗ Quartermaster (failed — try manually: curl stockyard.dev/quartermaster/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Notebook..."
  if curl -fsSL "https://stockyard.dev/notebook/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Notebook"
  else
    echo "    ✗ Notebook (failed — try manually: curl stockyard.dev/notebook/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Steward..."
  if curl -fsSL "https://stockyard.dev/steward/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Steward"
  else
    echo "    ✗ Steward (failed — try manually: curl stockyard.dev/steward/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 4 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
