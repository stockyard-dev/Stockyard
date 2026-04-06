#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Restaurants & Cafes"
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

BUNDLE_DIR="$HOME/stockyard-restaurant"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

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
echo "  Downloading Reservation..."
URL="https://github.com/stockyard-dev/stockyard-reservation/releases/latest/download/stockyard-reservation_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-reservation_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-reservation" 2>/dev/null || \
  mv "$TMP/stockyard-reservation" "$BUNDLE_DIR/tools/stockyard-reservation" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-reservation" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Reservation"
else
  echo "    ✗ Reservation (failed)"
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

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Restaurants & Cafes..."
echo ""
mkdir -p "$DATA/menu" && PORT=9812 "$DIR/tools/stockyard-menu" -port 9812 -data "$DATA/menu" >/dev/null 2>&1 &
mkdir -p "$DATA/reservation" && PORT=9806 "$DIR/tools/stockyard-reservation" -port 9806 -data "$DATA/reservation" >/dev/null 2>&1 &
mkdir -p "$DATA/quartermaster" && PORT=10230 "$DIR/tools/stockyard-quartermaster" -port 10230 -data "$DATA/quartermaster" >/dev/null 2>&1 &
mkdir -p "$DATA/steward" && PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA/steward" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/roster" && PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA/roster" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Menu                      http://localhost:9812/ui"
echo "  ✓ Reservation               http://localhost:9806/ui"
echo "  ✓ Quartermaster             http://localhost:10230/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:9812/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:9812/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-menu" 2>/dev/null && echo "  ✓ Stopped Menu" || true
pkill -f "stockyard-reservation" 2>/dev/null && echo "  ✓ Stopped Reservation" || true
pkill -f "stockyard-quartermaster" 2>/dev/null && echo "  ✓ Stopped Quartermaster" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR RESTAURANTS & CAFES

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Menu                      http://localhost:9812/ui
  Reservation               http://localhost:9806/ui
  Quartermaster             http://localhost:10230/ui
  Steward                   http://localhost:9840/ui
  Sundial                   http://localhost:9890/ui
  Roster                    http://localhost:8970/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=restaurant
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
