#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌────────────────────────────────────────────┐"
echo "  │  Stockyard for Barber Shops & Hair Salons  │"
echo "  │  8 tools · $7.99/mo · self-hosted          │"
echo "  │  https://stockyard.dev/for/barber-salon/   │"
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

  echo "  Installing Billfold..."
  if curl -fsSL "https://stockyard.dev/billfold/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Billfold"
  else
    echo "    ✗ Billfold (failed — try manually: curl stockyard.dev/billfold/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sundial..."
  if curl -fsSL "https://stockyard.dev/sundial/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sundial"
  else
    echo "    ✗ Sundial (failed — try manually: curl stockyard.dev/sundial/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Steward..."
  if curl -fsSL "https://stockyard.dev/steward/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Steward"
  else
    echo "    ✗ Steward (failed — try manually: curl stockyard.dev/steward/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Announcements..."
  if curl -fsSL "https://stockyard.dev/announcements/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Announcements"
  else
    echo "    ✗ Announcements (failed — try manually: curl stockyard.dev/announcements/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Surveyor..."
  if curl -fsSL "https://stockyard.dev/surveyor/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Surveyor"
  else
    echo "    ✗ Surveyor (failed — try manually: curl stockyard.dev/surveyor/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Pony Express..."
  if curl -fsSL "https://stockyard.dev/ponyexpress/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Pony Express"
  else
    echo "    ✗ Pony Express (failed — try manually: curl stockyard.dev/ponyexpress/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Booking..."
  if curl -fsSL "https://stockyard.dev/booking/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Booking"
  else
    echo "    ✗ Booking (failed — try manually: curl stockyard.dev/booking/install.sh | sh)"
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
