#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Music Schools             │"
echo "  │  7 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/music-school/  │"
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

  echo "  Installing Booking..."
  if curl -fsSL "https://stockyard.dev/booking/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Booking"
  else
    echo "    ✗ Booking (failed — try manually: curl stockyard.dev/booking/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Curriculum..."
  if curl -fsSL "https://stockyard.dev/curriculum/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Curriculum"
  else
    echo "    ✗ Curriculum (failed — try manually: curl stockyard.dev/curriculum/install.sh | sh)"
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

  echo "  Installing Tournament..."
  if curl -fsSL "https://stockyard.dev/tournament/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Tournament"
  else
    echo "    ✗ Tournament (failed — try manually: curl stockyard.dev/tournament/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Checkin..."
  if curl -fsSL "https://stockyard.dev/checkin/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Checkin"
  else
    echo "    ✗ Checkin (failed — try manually: curl stockyard.dev/checkin/install.sh | sh)"
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
