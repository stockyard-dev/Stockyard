#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Small Architecture Firms  │"
echo "  │  9 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/architect/    │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Dossier..."
  if curl -fsSL "https://stockyard.dev/dossier/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dossier"
  else
    echo "    ✗ Dossier (failed — try manually: curl stockyard.dev/dossier/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Billfold..."
  if curl -fsSL "https://stockyard.dev/billfold/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Billfold"
  else
    echo "    ✗ Billfold (failed — try manually: curl stockyard.dev/billfold/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Roundup..."
  if curl -fsSL "https://stockyard.dev/roundup/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Roundup"
  else
    echo "    ✗ Roundup (failed — try manually: curl stockyard.dev/roundup/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Steward..."
  if curl -fsSL "https://stockyard.dev/steward/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Steward"
  else
    echo "    ✗ Steward (failed — try manually: curl stockyard.dev/steward/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sundial..."
  if curl -fsSL "https://stockyard.dev/sundial/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sundial"
  else
    echo "    ✗ Sundial (failed — try manually: curl stockyard.dev/sundial/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Notebook..."
  if curl -fsSL "https://stockyard.dev/notebook/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Notebook"
  else
    echo "    ✗ Notebook (failed — try manually: curl stockyard.dev/notebook/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Deposition..."
  if curl -fsSL "https://stockyard.dev/deposition/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Deposition"
  else
    echo "    ✗ Deposition (failed — try manually: curl stockyard.dev/deposition/install.sh | sh)"
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
  echo "  ✓ All 9 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
