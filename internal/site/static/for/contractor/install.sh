#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for General Contractors       │"
echo "  │  7 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/contractor/   │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Estimate..."
  if curl -fsSL "https://stockyard.dev/estimate/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Estimate"
  else
    echo "    ✗ Estimate (failed — try manually: curl stockyard.dev/estimate/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Permit..."
  if curl -fsSL "https://stockyard.dev/permit/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Permit"
  else
    echo "    ✗ Permit (failed — try manually: curl stockyard.dev/permit/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Fleet..."
  if curl -fsSL "https://stockyard.dev/fleet/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Fleet"
  else
    echo "    ✗ Fleet (failed — try manually: curl stockyard.dev/fleet/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

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

  echo "  Installing Steward..."
  if curl -fsSL "https://stockyard.dev/steward/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Steward"
  else
    echo "    ✗ Steward (failed — try manually: curl stockyard.dev/steward/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Notebook..."
  if curl -fsSL "https://stockyard.dev/notebook/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Notebook"
  else
    echo "    ✗ Notebook (failed — try manually: curl stockyard.dev/notebook/install.sh | sh)"
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
