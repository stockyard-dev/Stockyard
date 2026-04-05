#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Board Game Groups        │"
echo "  │  6 tools · $7.99/mo · self-hosted        │"
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

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "  ✓ All 6 tools installed!"
else
  echo ""
  echo "  Installed 6 tools ($FAILED had issues)"
fi

echo ""
echo "  Each tool runs on its own port with a web dashboard at /ui"
echo "  Free tier: 5 items per tool. Upgrade: stockyard.dev/pricing/?bundle=board-gamer"
echo ""
echo "  Questions? hello@stockyard.dev"
echo ""
