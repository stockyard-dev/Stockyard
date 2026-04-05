#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Stockyard for Churches & Houses of Worship│"
echo "  │  8 tools · $7.99/mo · self-hosted        │"
echo "  │  https://stockyard.dev/for/church/       │"
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

  echo "  Installing Agora..."
  if curl -fsSL "https://stockyard.dev/agora/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Agora"
  else
    echo "    ✗ Agora (failed — try manually: curl stockyard.dev/agora/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Surveyor..."
  if curl -fsSL "https://stockyard.dev/surveyor/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Surveyor"
  else
    echo "    ✗ Surveyor (failed — try manually: curl stockyard.dev/surveyor/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Sundial..."
  if curl -fsSL "https://stockyard.dev/sundial/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Sundial"
  else
    echo "    ✗ Sundial (failed — try manually: curl stockyard.dev/sundial/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Pony Express..."
  if curl -fsSL "https://stockyard.dev/ponyexpress/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Pony Express"
  else
    echo "    ✗ Pony Express (failed — try manually: curl stockyard.dev/ponyexpress/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

  echo "  Installing Roster..."
  if curl -fsSL "https://stockyard.dev/roster/install.sh" 2>/dev/null | sh >/dev/null 2>&1; then
    echo "    ✓ Roster"
  else
    echo "    ✗ Roster (failed — try manually: curl stockyard.dev/roster/install.sh | sh)"
    FAILED=$((FAILED + 1))
  fi

if [ "$FAILED" -eq 0 ]; then
  echo ""
  echo "  ✓ All 8 tools installed!"
else
  echo ""
  echo "  Installed 8 tools ($FAILED had issues)"
fi

echo ""
echo "  Each tool runs on its own port with a web dashboard at /ui"
echo "  Free tier: 5 items per tool. Upgrade: stockyard.dev/pricing/?bundle=church"
echo ""
echo "  Questions? hello@stockyard.dev"
echo ""
