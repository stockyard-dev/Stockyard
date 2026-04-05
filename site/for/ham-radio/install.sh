#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Amateur Radio Operators  │"
echo "  │  6 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/ham-radio/    │"
echo "  └──────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Campfire..."
  if curl -fsSL "https://stockyard.dev/campfire/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Campfire"
  else
    echo "    ✗ Campfire (failed — try manually: curl stockyard.dev/campfire/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Dossier..."
  if curl -fsSL "https://stockyard.dev/dossier/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dossier"
  else
    echo "    ✗ Dossier (failed — try manually: curl stockyard.dev/dossier/install.sh | sh)"
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

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "  ✓ All 6 tools installed!"
else
  echo ""
  echo "  Installed 6 tools ($FAILED had issues)"
fi

echo ""
echo "  Each tool runs on its own port with a web dashboard at /ui"
echo "  Free tier: 5 items per tool. Upgrade: stockyard.dev/pricing/?bundle=ham-radio"
echo ""
echo "  Questions? hello@stockyard.dev"
echo ""
