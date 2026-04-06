#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Book Clubs & Reading Groups"
echo "  6 tools — self-hosted on your hardware"
echo ""

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "  Unsupported architecture: $ARCH"; exit 1 ;;
esac
echo "  Platform: $OS/$ARCH"
echo ""

BUNDLE_DIR="$HOME/stockyard-bookclub"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

echo "  Downloading Roster..."
URL="https://github.com/stockyard-dev/stockyard-roster/releases/latest/download/stockyard-roster_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-roster_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-roster" 2>/dev/null || \
  mv "$TMP/stockyard-roster" "$BUNDLE_DIR/tools/stockyard-roster" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-roster" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Roster"
else
  echo "    ✗ Roster (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Sundial..."
URL="https://github.com/stockyard-dev/stockyard-sundial/releases/latest/download/stockyard-sundial_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-sundial_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-sundial" 2>/dev/null || \
  mv "$TMP/stockyard-sundial" "$BUNDLE_DIR/tools/stockyard-sundial" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-sundial" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Sundial"
else
  echo "    ✗ Sundial (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Agora..."
URL="https://github.com/stockyard-dev/stockyard-agora/releases/latest/download/stockyard-agora_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-agora_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-agora" 2>/dev/null || \
  mv "$TMP/stockyard-agora" "$BUNDLE_DIR/tools/stockyard-agora" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-agora" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Agora"
else
  echo "    ✗ Agora (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Announcements..."
URL="https://github.com/stockyard-dev/stockyard-announcements/releases/latest/download/stockyard-announcements_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-announcements_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-announcements" 2>/dev/null || \
  mv "$TMP/stockyard-announcements" "$BUNDLE_DIR/tools/stockyard-announcements" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-announcements" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Announcements"
else
  echo "    ✗ Announcements (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Notebook..."
URL="https://github.com/stockyard-dev/stockyard-notebook/releases/latest/download/stockyard-notebook_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-notebook_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-notebook" 2>/dev/null || \
  mv "$TMP/stockyard-notebook" "$BUNDLE_DIR/tools/stockyard-notebook" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-notebook" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Notebook"
else
  echo "    ✗ Notebook (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Dossier..."
URL="https://github.com/stockyard-dev/stockyard-dossier/releases/latest/download/stockyard-dossier_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-dossier_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-dossier" 2>/dev/null || \
  mv "$TMP/stockyard-dossier" "$BUNDLE_DIR/tools/stockyard-dossier" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-dossier" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Dossier"
else
  echo "    ✗ Dossier (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Book Clubs & Reading Groups..."
echo ""
mkdir -p "$DATA/roster" && PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA/roster" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/agora" && PORT=10070 "$DIR/tools/stockyard-agora" -port 10070 -data "$DATA/agora" >/dev/null 2>&1 &
mkdir -p "$DATA/announcements" && PORT=9750 "$DIR/tools/stockyard-announcements" -port 9750 -data "$DATA/announcements" >/dev/null 2>&1 &
mkdir -p "$DATA/notebook" && PORT=9370 "$DIR/tools/stockyard-notebook" -port 9370 -data "$DATA/notebook" >/dev/null 2>&1 &
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Roster                    http://localhost:8970/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Agora                     http://localhost:10070/ui"
echo "  ✓ Announcements             http://localhost:9750/ui"
echo "  ✓ Notebook                  http://localhost:9370/ui"
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:8970/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:8970/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-agora" 2>/dev/null && echo "  ✓ Stopped Agora" || true
pkill -f "stockyard-announcements" 2>/dev/null && echo "  ✓ Stopped Announcements" || true
pkill -f "stockyard-notebook" 2>/dev/null && echo "  ✓ Stopped Notebook" || true
pkill -f "stockyard-dossier" 2>/dev/null && echo "  ✓ Stopped Dossier" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR BOOK CLUBS & READING GROUPS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Roster                    http://localhost:8970/ui
  Sundial                   http://localhost:9890/ui
  Agora                     http://localhost:10070/ui
  Announcements             http://localhost:9750/ui
  Notebook                  http://localhost:9370/ui
  Dossier                   http://localhost:9080/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=bookclub
Help:    hello@stockyard.dev
READMEEOF

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 6 tools installed to $BUNDLE_DIR/"
else
  echo "  ⚠ $FAILED tool(s) failed. The rest are ready."
fi
echo ""
echo "  Next steps:"
echo "    cd $BUNDLE_DIR"
echo "    ./start.sh"
echo ""
