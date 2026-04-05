#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌────────────────────────────────────────────┐"
echo "  │  Stockyard for Twitch & YouTube Streamers  │"
echo "  │  6 tools · $7.99/mo · self-hosted          │"
echo "  │  https://stockyard.dev/for/streamer/       │"
echo "  └────────────────────────────────────────────┘"
echo ""

FAILED=0

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

  echo "  Installing Billfold..."
  if curl -fsSL "https://stockyard.dev/billfold/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Billfold"
  else
    echo "    ✗ Billfold (failed — try manually: curl stockyard.dev/billfold/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Portfolio..."
  if curl -fsSL "https://stockyard.dev/portfolio/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Portfolio"
  else
    echo "    ✗ Portfolio (failed — try manually: curl stockyard.dev/portfolio/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Prospector..."
  if curl -fsSL "https://stockyard.dev/prospector/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Prospector"
  else
    echo "    ✗ Prospector (failed — try manually: curl stockyard.dev/prospector/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Dispatch..."
  if curl -fsSL "https://stockyard.dev/dispatch/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dispatch"
  else
    echo "    ✗ Dispatch (failed — try manually: curl stockyard.dev/dispatch/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 6 tools installed successfully!"
else
  echo "  ⚠ $FAILED tool(s) failed. Check the output above."
fi
echo ""
echo "  Dashboard: run any tool and open http://localhost:<port>/ui"
echo "  Questions? hello@stockyard.dev"
echo ""
