#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌───────────────────────────────────────────────────────┐"
echo "  │  Stockyard for MTG & Card Game Tournament Organizers  │"
echo "  │  4 tools · $7.99/mo · self-hosted                     │"
echo "  │  https://stockyard.dev/for/mtg/                       │"
echo "  └───────────────────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Tournament..."
  if curl -fsSL "https://stockyard.dev/tournament/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tournament"
  else
    echo "    ✗ Tournament (failed — try manually: curl stockyard.dev/tournament/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Dossier..."
  if curl -fsSL "https://stockyard.dev/dossier/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dossier"
  else
    echo "    ✗ Dossier (failed — try manually: curl stockyard.dev/dossier/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Surveyor..."
  if curl -fsSL "https://stockyard.dev/surveyor/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Surveyor"
  else
    echo "    ✗ Surveyor (failed — try manually: curl stockyard.dev/surveyor/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Tally..."
  if curl -fsSL "https://stockyard.dev/tally/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tally"
  else
    echo "    ✗ Tally (failed — try manually: curl stockyard.dev/tally/install.sh | sh)"
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
