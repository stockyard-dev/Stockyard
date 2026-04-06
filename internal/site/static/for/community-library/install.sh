#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Community & Little Free Libraries"
echo "  8 tools — self-hosted on your hardware"
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

BUNDLE_DIR="$HOME/stockyard-community-library"
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
echo "  Downloading Tally..."
URL="https://github.com/stockyard-dev/stockyard-tally/releases/latest/download/stockyard-tally_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-tally_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-tally" 2>/dev/null || \
  mv "$TMP/stockyard-tally" "$BUNDLE_DIR/tools/stockyard-tally" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-tally" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Tally"
else
  echo "    ✗ Tally (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Checkout..."
URL="https://github.com/stockyard-dev/stockyard-checkout/releases/latest/download/stockyard-checkout_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-checkout_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-checkout" 2>/dev/null || \
  mv "$TMP/stockyard-checkout" "$BUNDLE_DIR/tools/stockyard-checkout" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-checkout" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Checkout"
else
  echo "    ✗ Checkout (failed)"
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

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Community & Little Free Libraries..."
echo ""
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
mkdir -p "$DATA/quartermaster" && PORT=10230 "$DIR/tools/stockyard-quartermaster" -port 10230 -data "$DATA/quartermaster" >/dev/null 2>&1 &
mkdir -p "$DATA/roster" && PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA/roster" >/dev/null 2>&1 &
mkdir -p "$DATA/announcements" && PORT=9750 "$DIR/tools/stockyard-announcements" -port 9750 -data "$DATA/announcements" >/dev/null 2>&1 &
mkdir -p "$DATA/surveyor" && PORT=9290 "$DIR/tools/stockyard-surveyor" -port 9290 -data "$DATA/surveyor" >/dev/null 2>&1 &
mkdir -p "$DATA/tally" && PORT=8640 "$DIR/tools/stockyard-tally" -port 8640 -data "$DATA/tally" >/dev/null 2>&1 &
mkdir -p "$DATA/checkout" && PORT=9100 "$DIR/tools/stockyard-checkout" -port 9100 -data "$DATA/checkout" >/dev/null 2>&1 &
mkdir -p "$DATA/reservation" && PORT=9806 "$DIR/tools/stockyard-reservation" -port 9806 -data "$DATA/reservation" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Quartermaster             http://localhost:10230/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo "  ✓ Announcements             http://localhost:9750/ui"
echo "  ✓ Surveyor                  http://localhost:9290/ui"
echo "  ✓ Tally                     http://localhost:8640/ui"
echo "  ✓ Checkout                  http://localhost:9100/ui"
echo "  ✓ Reservation               http://localhost:9806/ui"
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
pkill -f "stockyard-quartermaster" 2>/dev/null && echo "  ✓ Stopped Quartermaster" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
pkill -f "stockyard-announcements" 2>/dev/null && echo "  ✓ Stopped Announcements" || true
pkill -f "stockyard-surveyor" 2>/dev/null && echo "  ✓ Stopped Surveyor" || true
pkill -f "stockyard-tally" 2>/dev/null && echo "  ✓ Stopped Tally" || true
pkill -f "stockyard-checkout" 2>/dev/null && echo "  ✓ Stopped Checkout" || true
pkill -f "stockyard-reservation" 2>/dev/null && echo "  ✓ Stopped Reservation" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR COMMUNITY & LITTLE FREE LIBRARIES

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Quartermaster             http://localhost:10230/ui
  Roster                    http://localhost:8970/ui
  Announcements             http://localhost:9750/ui
  Surveyor                  http://localhost:9290/ui
  Tally                     http://localhost:8640/ui
  Checkout                  http://localhost:9100/ui
  Reservation               http://localhost:9806/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=community-library
Help:    hello@stockyard.dev
READMEEOF

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 8 tools installed to $BUNDLE_DIR/"
else
  echo "  ⚠ $FAILED tool(s) failed. The rest are ready."
fi
echo ""
echo "  Next steps:"
echo "    cd $BUNDLE_DIR"
echo "    ./start.sh"
echo ""
