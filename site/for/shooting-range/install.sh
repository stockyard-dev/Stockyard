#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌─────────────────────────────────────────────┐"
echo "  │  Stockyard for Shooting Ranges & Gun Clubs  │"
echo "  │  6 tools · $7.99/mo · self-hosted           │"
echo "  │  https://stockyard.dev/for/shooting-range/  │"
echo "  └─────────────────────────────────────────────┘"
echo ""

FAILED=0

  echo "  Installing Dossier..."
  if curl -fsSL "https://stockyard.dev/dossier/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Dossier"
  else
    echo "    ✗ Dossier (failed — try manually: curl stockyard.dev/dossier/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Waiver..."
  if curl -fsSL "https://stockyard.dev/waiver/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Waiver"
  else
    echo "    ✗ Waiver (failed — try manually: curl stockyard.dev/waiver/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Checkin..."
  if curl -fsSL "https://stockyard.dev/checkin/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Checkin"
  else
    echo "    ✗ Checkin (failed — try manually: curl stockyard.dev/checkin/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Reservation..."
  if curl -fsSL "https://stockyard.dev/reservation/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Reservation"
  else
    echo "    ✗ Reservation (failed — try manually: curl stockyard.dev/reservation/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Permit..."
  if curl -fsSL "https://stockyard.dev/permit/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Permit"
  else
    echo "    ✗ Permit (failed — try manually: curl stockyard.dev/permit/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Billfold..."
  if curl -fsSL "https://stockyard.dev/billfold/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Billfold"
  else
    echo "    ✗ Billfold (failed — try manually: curl stockyard.dev/billfold/install.sh | sh)"
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
