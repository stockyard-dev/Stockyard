#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Food Trucks               │"
echo "  │  11 tools · $7.99/mo · self-hosted       │"
echo "  │  https://stockyard.dev/for/food-truck/   │"
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

  echo "  Installing Steward..."
  if curl -fsSL "https://stockyard.dev/steward/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Steward"
  else
    echo "    ✗ Steward (failed — try manually: curl stockyard.dev/steward/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Billfold..."
  if curl -fsSL "https://stockyard.dev/billfold/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Billfold"
  else
    echo "    ✗ Billfold (failed — try manually: curl stockyard.dev/billfold/install.sh | sh)"
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

  echo "  Installing Announcements..."
  if curl -fsSL "https://stockyard.dev/announcements/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Announcements"
  else
    echo "    ✗ Announcements (failed — try manually: curl stockyard.dev/announcements/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Tally..."
  if curl -fsSL "https://stockyard.dev/tally/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tally"
  else
    echo "    ✗ Tally (failed — try manually: curl stockyard.dev/tally/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Fleet..."
  if curl -fsSL "https://stockyard.dev/fleet/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Fleet"
  else
    echo "    ✗ Fleet (failed — try manually: curl stockyard.dev/fleet/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Menu..."
  if curl -fsSL "https://stockyard.dev/menu/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Menu"
  else
    echo "    ✗ Menu (failed — try manually: curl stockyard.dev/menu/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Permit..."
  if curl -fsSL "https://stockyard.dev/permit/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Permit"
  else
    echo "    ✗ Permit (failed — try manually: curl stockyard.dev/permit/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Recipe..."
  if curl -fsSL "https://stockyard.dev/recipe/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Recipe"
  else
    echo "    ✗ Recipe (failed — try manually: curl stockyard.dev/recipe/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 11 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
