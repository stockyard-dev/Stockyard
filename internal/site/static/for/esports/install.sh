#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌────────────────────────────────────────────┐"
echo "  │  Stockyard for Esports Teams & Organizers  │"
echo "  │  6 tools · $7.99/mo · self-hosted          │"
echo "  │  https://stockyard.dev/for/esports/        │"
echo "  └────────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Tournament..."
  if curl -fsSL "https://stockyard.dev/tournament/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tournament"
  else
    echo "    ✗ Tournament (failed — try manually: curl stockyard.dev/tournament/install.sh | sh)"
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

  echo "  Installing Campfire..."
  if curl -fsSL "https://stockyard.dev/campfire/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Campfire"
  else
    echo "    ✗ Campfire (failed — try manually: curl stockyard.dev/campfire/install.sh | sh)"
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
