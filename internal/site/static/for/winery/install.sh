#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Wineries & Vineyards"
echo "  7 tools — self-hosted on your hardware"
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

BUNDLE_DIR="$HOME/stockyard-winery"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

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
echo "  Downloading Recipe..."
URL="https://github.com/stockyard-dev/stockyard-recipe/releases/latest/download/stockyard-recipe_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-recipe_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-recipe" 2>/dev/null || \
  mv "$TMP/stockyard-recipe" "$BUNDLE_DIR/tools/stockyard-recipe" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-recipe" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Recipe"
else
  echo "    ✗ Recipe (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Menu..."
URL="https://github.com/stockyard-dev/stockyard-menu/releases/latest/download/stockyard-menu_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-menu_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-menu" 2>/dev/null || \
  mv "$TMP/stockyard-menu" "$BUNDLE_DIR/tools/stockyard-menu" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-menu" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Menu"
else
  echo "    ✗ Menu (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Quartermaster..."
URL="https://github.com/stockyard-dev/stockyard-quartermaster/releases/latest/download/stockyard-quartermaster_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-quartermaster_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-quartermaster" 2>/dev/null || \
  mv "$TMP/stockyard-quartermaster" "$BUNDLE_DIR/tools/stockyard-quartermaster" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-quartermaster" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Quartermaster"
else
  echo "    ✗ Quartermaster (failed)"
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
echo "  Downloading Booking..."
URL="https://github.com/stockyard-dev/stockyard-booking/releases/latest/download/stockyard-booking_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-booking_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-booking" 2>/dev/null || \
  mv "$TMP/stockyard-booking" "$BUNDLE_DIR/tools/stockyard-booking" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-booking" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Booking"
else
  echo "    ✗ Booking (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Wineries & Vineyards..."
echo ""
mkdir -p "$DATA/harvest" && PORT=9810 "$DIR/tools/stockyard-harvest" -port 9810 -data "$DATA/harvest" >/dev/null 2>&1 &
mkdir -p "$DATA/recipe" && PORT=9805 "$DIR/tools/stockyard-recipe" -port 9805 -data "$DATA/recipe" >/dev/null 2>&1 &
mkdir -p "$DATA/menu" && PORT=9812 "$DIR/tools/stockyard-menu" -port 9812 -data "$DATA/menu" >/dev/null 2>&1 &
mkdir -p "$DATA/quartermaster" && PORT=10230 "$DIR/tools/stockyard-quartermaster" -port 10230 -data "$DATA/quartermaster" >/dev/null 2>&1 &
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
mkdir -p "$DATA/steward" && PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA/steward" >/dev/null 2>&1 &
mkdir -p "$DATA/booking" && PORT=9800 "$DIR/tools/stockyard-booking" -port 9800 -data "$DATA/booking" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Harvest                   http://localhost:9810/ui"
echo "  ✓ Recipe                    http://localhost:9805/ui"
echo "  ✓ Menu                      http://localhost:9812/ui"
echo "  ✓ Quartermaster             http://localhost:10230/ui"
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo "  ✓ Booking                   http://localhost:9800/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:9810/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:9810/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-harvest" 2>/dev/null && echo "  ✓ Stopped Harvest" || true
pkill -f "stockyard-recipe" 2>/dev/null && echo "  ✓ Stopped Recipe" || true
pkill -f "stockyard-menu" 2>/dev/null && echo "  ✓ Stopped Menu" || true
pkill -f "stockyard-quartermaster" 2>/dev/null && echo "  ✓ Stopped Quartermaster" || true
pkill -f "stockyard-dossier" 2>/dev/null && echo "  ✓ Stopped Dossier" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
pkill -f "stockyard-booking" 2>/dev/null && echo "  ✓ Stopped Booking" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR WINERIES & VINEYARDS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Harvest                   http://localhost:9810/ui
  Recipe                    http://localhost:9805/ui
  Menu                      http://localhost:9812/ui
  Quartermaster             http://localhost:10230/ui
  Dossier                   http://localhost:9080/ui
  Steward                   http://localhost:9840/ui
  Booking                   http://localhost:9800/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=winery
Help:    hello@stockyard.dev
READMEEOF

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 7 tools installed to $BUNDLE_DIR/"
else
  echo "  ⚠ $FAILED tool(s) failed. The rest are ready."
fi
echo ""
echo "  Next steps:"
echo "    cd $BUNDLE_DIR"
echo "    ./start.sh"
echo ""
