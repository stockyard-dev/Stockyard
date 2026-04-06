#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Small Museums & Historical Societies"
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

BUNDLE_DIR="$HOME/stockyard-museum"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

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
echo "  Downloading Checkin..."
URL="https://github.com/stockyard-dev/stockyard-checkin/releases/latest/download/stockyard-checkin_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-checkin_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-checkin" 2>/dev/null || \
  mv "$TMP/stockyard-checkin" "$BUNDLE_DIR/tools/stockyard-checkin" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-checkin" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Checkin"
else
  echo "    ✗ Checkin (failed)"
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
echo "  Downloading Portfolio..."
URL="https://github.com/stockyard-dev/stockyard-portfolio/releases/latest/download/stockyard-portfolio_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-portfolio_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-portfolio" 2>/dev/null || \
  mv "$TMP/stockyard-portfolio" "$BUNDLE_DIR/tools/stockyard-portfolio" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-portfolio" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Portfolio"
else
  echo "    ✗ Portfolio (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Surveyor..."
URL="https://github.com/stockyard-dev/stockyard-surveyor/releases/latest/download/stockyard-surveyor_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-surveyor_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-surveyor" 2>/dev/null || \
  mv "$TMP/stockyard-surveyor" "$BUNDLE_DIR/tools/stockyard-surveyor" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-surveyor" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Surveyor"
else
  echo "    ✗ Surveyor (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Small Museums & Historical Societies..."
echo ""
PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA" >/dev/null 2>&1 &
PORT=9807 "$DIR/tools/stockyard-checkin" -port 9807 -data "$DATA" >/dev/null 2>&1 &
PORT=9800 "$DIR/tools/stockyard-booking" -port 9800 -data "$DATA" >/dev/null 2>&1 &
PORT=9070 "$DIR/tools/stockyard-billfold" -port 9070 -data "$DATA" >/dev/null 2>&1 &
PORT=9808 "$DIR/tools/stockyard-portfolio" -port 9808 -data "$DATA" >/dev/null 2>&1 &
PORT=9290 "$DIR/tools/stockyard-surveyor" -port 9290 -data "$DATA" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Checkin                   http://localhost:9807/ui"
echo "  ✓ Booking                   http://localhost:9800/ui"
echo "  ✓ Billfold                  http://localhost:9070/ui"
echo "  ✓ Portfolio                 http://localhost:9808/ui"
echo "  ✓ Surveyor                  http://localhost:9290/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:9080/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:9080/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-dossier" 2>/dev/null && echo "  ✓ Stopped Dossier" || true
pkill -f "stockyard-checkin" 2>/dev/null && echo "  ✓ Stopped Checkin" || true
pkill -f "stockyard-booking" 2>/dev/null && echo "  ✓ Stopped Booking" || true
pkill -f "stockyard-billfold" 2>/dev/null && echo "  ✓ Stopped Billfold" || true
pkill -f "stockyard-portfolio" 2>/dev/null && echo "  ✓ Stopped Portfolio" || true
pkill -f "stockyard-surveyor" 2>/dev/null && echo "  ✓ Stopped Surveyor" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR SMALL MUSEUMS & HISTORICAL SOCIETIES

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Checkin                   http://localhost:9807/ui
  Booking                   http://localhost:9800/ui
  Billfold                  http://localhost:9070/ui
  Portfolio                 http://localhost:9808/ui
  Surveyor                  http://localhost:9290/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=museum
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
