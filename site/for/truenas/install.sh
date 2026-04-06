#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for TrueNAS Users"
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

BUNDLE_DIR="$HOME/stockyard-truenas"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

echo "  Downloading Sentinel..."
URL="https://github.com/stockyard-dev/stockyard-sentinel/releases/latest/download/stockyard-sentinel_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-sentinel_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-sentinel" 2>/dev/null || \
  mv "$TMP/stockyard-sentinel" "$BUNDLE_DIR/tools/stockyard-sentinel" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-sentinel" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Sentinel"
else
  echo "    ✗ Sentinel (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Outpost..."
URL="https://github.com/stockyard-dev/stockyard-outpost/releases/latest/download/stockyard-outpost_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-outpost_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-outpost" 2>/dev/null || \
  mv "$TMP/stockyard-outpost" "$BUNDLE_DIR/tools/stockyard-outpost" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-outpost" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Outpost"
else
  echo "    ✗ Outpost (failed)"
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
echo "  Downloading Silo..."
URL="https://github.com/stockyard-dev/stockyard-silo/releases/latest/download/stockyard-silo_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-silo_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-silo" 2>/dev/null || \
  mv "$TMP/stockyard-silo" "$BUNDLE_DIR/tools/stockyard-silo" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-silo" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Silo"
else
  echo "    ✗ Silo (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Mainspring..."
URL="https://github.com/stockyard-dev/stockyard-mainspring/releases/latest/download/stockyard-mainspring_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-mainspring_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-mainspring" 2>/dev/null || \
  mv "$TMP/stockyard-mainspring" "$BUNDLE_DIR/tools/stockyard-mainspring" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-mainspring" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Mainspring"
else
  echo "    ✗ Mainspring (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for TrueNAS Users..."
echo ""
mkdir -p "$DATA/sentinel" && PORT=8680 "$DIR/tools/stockyard-sentinel" -port 8680 -data "$DATA/sentinel" >/dev/null 2>&1 &
mkdir -p "$DATA/outpost" && PORT=8740 "$DIR/tools/stockyard-outpost" -port 8740 -data "$DATA/outpost" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/tally" && PORT=8640 "$DIR/tools/stockyard-tally" -port 8640 -data "$DATA/tally" >/dev/null 2>&1 &
mkdir -p "$DATA/silo" && PORT=8545 "$DIR/tools/stockyard-silo" -port 8545 -data "$DATA/silo" >/dev/null 2>&1 &
mkdir -p "$DATA/mainspring" && PORT=9950 "$DIR/tools/stockyard-mainspring" -port 9950 -data "$DATA/mainspring" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Sentinel                  http://localhost:8680/ui"
echo "  ✓ Outpost                   http://localhost:8740/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Tally                     http://localhost:8640/ui"
echo "  ✓ Silo                      http://localhost:8545/ui"
echo "  ✓ Mainspring                http://localhost:9950/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:8680/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:8680/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-sentinel" 2>/dev/null && echo "  ✓ Stopped Sentinel" || true
pkill -f "stockyard-outpost" 2>/dev/null && echo "  ✓ Stopped Outpost" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-tally" 2>/dev/null && echo "  ✓ Stopped Tally" || true
pkill -f "stockyard-silo" 2>/dev/null && echo "  ✓ Stopped Silo" || true
pkill -f "stockyard-mainspring" 2>/dev/null && echo "  ✓ Stopped Mainspring" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR TRUENAS USERS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Sentinel                  http://localhost:8680/ui
  Outpost                   http://localhost:8740/ui
  Sundial                   http://localhost:9890/ui
  Tally                     http://localhost:8640/ui
  Silo                      http://localhost:8545/ui
  Mainspring                http://localhost:9950/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=truenas
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
