#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Woodworkers               │"
echo "  │  8 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/woodworker/   │"
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

  echo "  Installing Roundup..."
  if curl -fsSL "https://stockyard.dev/roundup/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Roundup"
  else
    echo "    ✗ Roundup (failed — try manually: curl stockyard.dev/roundup/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Tally..."
  if curl -fsSL "https://stockyard.dev/tally/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tally"
  else
    echo "    ✗ Tally (failed — try manually: curl stockyard.dev/tally/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Collection..."
  if curl -fsSL "https://stockyard.dev/collection/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Collection"
  else
    echo "    ✗ Collection (failed — try manually: curl stockyard.dev/collection/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Estimate..."
  if curl -fsSL "https://stockyard.dev/estimate/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Estimate"
  else
    echo "    ✗ Estimate (failed — try manually: curl stockyard.dev/estimate/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Portfolio..."
  if curl -fsSL "https://stockyard.dev/portfolio/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Portfolio"
  else
    echo "    ✗ Portfolio (failed — try manually: curl stockyard.dev/portfolio/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 8 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
