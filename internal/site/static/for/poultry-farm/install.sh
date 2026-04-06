#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Poultry Farmers & Backyard Chickens"
echo "  5 tools — self-hosted on your hardware"
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

BUNDLE_DIR="$HOME/stockyard-poultry-farm"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

echo "  Downloading Breeding..."
URL="https://github.com/stockyard-dev/stockyard-breeding/releases/latest/download/stockyard-breeding_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-breeding_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-breeding" 2>/dev/null || \
  mv "$TMP/stockyard-breeding" "$BUNDLE_DIR/tools/stockyard-breeding" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-breeding" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Breeding"
else
  echo "    ✗ Breeding (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Harvest..."
URL="https://github.com/stockyard-dev/stockyard-harvest/releases/latest/download/stockyard-harvest_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-harvest_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-harvest" 2>/dev/null || \
  mv "$TMP/stockyard-harvest" "$BUNDLE_DIR/tools/stockyard-harvest" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-harvest" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Harvest"
else
  echo "    ✗ Harvest (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Steward..."
URL="https://github.com/stockyard-dev/stockyard-steward/releases/latest/download/stockyard-steward_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-steward_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-steward" 2>/dev/null || \
  mv "$TMP/stockyard-steward" "$BUNDLE_DIR/tools/stockyard-steward" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-steward" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Steward"
else
  echo "    ✗ Steward (failed)"
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
echo "  Starting Stockyard for Poultry Farmers & Backyard Chickens..."
echo ""
PORT=9803 "$DIR/tools/stockyard-breeding" -port 9803 -data "$DATA" >/dev/null 2>&1 &
PORT=9810 "$DIR/tools/stockyard-harvest" -port 9810 -data "$DATA" >/dev/null 2>&1 &
PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA" >/dev/null 2>&1 &
PORT=9370 "$DIR/tools/stockyard-notebook" -port 9370 -data "$DATA" >/dev/null 2>&1 &
PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Breeding                  http://localhost:9803/ui"
echo "  ✓ Harvest                   http://localhost:9810/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo "  ✓ Notebook                  http://localhost:9370/ui"
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:9803/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:9803/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-breeding" 2>/dev/null && echo "  ✓ Stopped Breeding" || true
pkill -f "stockyard-harvest" 2>/dev/null && echo "  ✓ Stopped Harvest" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
pkill -f "stockyard-notebook" 2>/dev/null && echo "  ✓ Stopped Notebook" || true
pkill -f "stockyard-dossier" 2>/dev/null && echo "  ✓ Stopped Dossier" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR POULTRY FARMERS & BACKYARD CHICKENS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Breeding                  http://localhost:9803/ui
  Harvest                   http://localhost:9810/ui
  Steward                   http://localhost:9840/ui
  Notebook                  http://localhost:9370/ui
  Dossier                   http://localhost:9080/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=poultry-farm
Help:    hello@stockyard.dev
READMEEOF

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 5 tools installed to $BUNDLE_DIR/"
else
  echo "  ⚠ $FAILED tool(s) failed. The rest are ready."
fi
echo ""
echo "  Next steps:"
echo "    cd $BUNDLE_DIR"
echo "    ./start.sh"
echo ""
