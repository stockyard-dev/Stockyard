#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Pressure Washing & Exterior Cleaning"
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

BUNDLE_DIR="$HOME/stockyard-pressure-washing"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

echo "  Downloading Estimate..."
URL="https://github.com/stockyard-dev/stockyard-estimate/releases/latest/download/stockyard-estimate_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-estimate_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-estimate" 2>/dev/null || \
  mv "$TMP/stockyard-estimate" "$BUNDLE_DIR/tools/stockyard-estimate" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-estimate" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Estimate"
else
  echo "    ✗ Estimate (failed)"
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
echo "  Downloading Fleet..."
URL="https://github.com/stockyard-dev/stockyard-fleet/releases/latest/download/stockyard-fleet_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-fleet_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-fleet" 2>/dev/null || \
  mv "$TMP/stockyard-fleet" "$BUNDLE_DIR/tools/stockyard-fleet" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-fleet" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Fleet"
else
  echo "    ✗ Fleet (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Billfold..."
URL="https://github.com/stockyard-dev/stockyard-billfold/releases/latest/download/stockyard-billfold_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-billfold_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-billfold" 2>/dev/null || \
  mv "$TMP/stockyard-billfold" "$BUNDLE_DIR/tools/stockyard-billfold" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-billfold" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Billfold"
else
  echo "    ✗ Billfold (failed)"
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

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Pressure Washing & Exterior Cleaning..."
echo ""
PORT=9802 "$DIR/tools/stockyard-estimate" -port 9802 -data "$DATA" >/dev/null 2>&1 &
PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA" >/dev/null 2>&1 &
PORT=9800 "$DIR/tools/stockyard-booking" -port 9800 -data "$DATA" >/dev/null 2>&1 &
PORT=9809 "$DIR/tools/stockyard-fleet" -port 9809 -data "$DATA" >/dev/null 2>&1 &
PORT=9070 "$DIR/tools/stockyard-billfold" -port 9070 -data "$DATA" >/dev/null 2>&1 &
PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Estimate                  http://localhost:9802/ui"
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Booking                   http://localhost:9800/ui"
echo "  ✓ Fleet                     http://localhost:9809/ui"
echo "  ✓ Billfold                  http://localhost:9070/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:9802/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:9802/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-estimate" 2>/dev/null && echo "  ✓ Stopped Estimate" || true
pkill -f "stockyard-dossier" 2>/dev/null && echo "  ✓ Stopped Dossier" || true
pkill -f "stockyard-booking" 2>/dev/null && echo "  ✓ Stopped Booking" || true
pkill -f "stockyard-fleet" 2>/dev/null && echo "  ✓ Stopped Fleet" || true
pkill -f "stockyard-billfold" 2>/dev/null && echo "  ✓ Stopped Billfold" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR PRESSURE WASHING & EXTERIOR CLEANING

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Estimate                  http://localhost:9802/ui
  Dossier                   http://localhost:9080/ui
  Booking                   http://localhost:9800/ui
  Fleet                     http://localhost:9809/ui
  Billfold                  http://localhost:9070/ui
  Steward                   http://localhost:9840/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=pressure-washing
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
