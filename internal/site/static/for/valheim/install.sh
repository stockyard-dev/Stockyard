#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Valheim Server Admins"
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

BUNDLE_DIR="$HOME/stockyard-valheim"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

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
echo "  Downloading Headcount..."
URL="https://github.com/stockyard-dev/stockyard-headcount/releases/latest/download/stockyard-headcount_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-headcount_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-headcount" 2>/dev/null || \
  mv "$TMP/stockyard-headcount" "$BUNDLE_DIR/tools/stockyard-headcount" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-headcount" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Headcount"
else
  echo "    ✗ Headcount (failed)"
  FAILED=$((FAILED + 1))
fi
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
echo "  Starting Stockyard for Valheim Server Admins..."
echo ""
PORT=8545 "$DIR/tools/stockyard-silo" -port 8545 -data "$DATA" >/dev/null 2>&1 &
PORT=8690 "$DIR/tools/stockyard-headcount" -port 8690 -data "$DATA" >/dev/null 2>&1 &
PORT=8680 "$DIR/tools/stockyard-sentinel" -port 8680 -data "$DATA" >/dev/null 2>&1 &
PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA" >/dev/null 2>&1 &
PORT=8740 "$DIR/tools/stockyard-outpost" -port 8740 -data "$DATA" >/dev/null 2>&1 &
PORT=9750 "$DIR/tools/stockyard-announcements" -port 9750 -data "$DATA" >/dev/null 2>&1 &
PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Silo                      http://localhost:8545/ui"
echo "  ✓ Headcount                 http://localhost:8690/ui"
echo "  ✓ Sentinel                  http://localhost:8680/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Outpost                   http://localhost:8740/ui"
echo "  ✓ Announcements             http://localhost:9750/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:8545/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:8545/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-silo" 2>/dev/null && echo "  ✓ Stopped Silo" || true
pkill -f "stockyard-headcount" 2>/dev/null && echo "  ✓ Stopped Headcount" || true
pkill -f "stockyard-sentinel" 2>/dev/null && echo "  ✓ Stopped Sentinel" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-outpost" 2>/dev/null && echo "  ✓ Stopped Outpost" || true
pkill -f "stockyard-announcements" 2>/dev/null && echo "  ✓ Stopped Announcements" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR VALHEIM SERVER ADMINS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Silo                      http://localhost:8545/ui
  Headcount                 http://localhost:8690/ui
  Sentinel                  http://localhost:8680/ui
  Sundial                   http://localhost:9890/ui
  Outpost                   http://localhost:8740/ui
  Announcements             http://localhost:9750/ui
  Roster                    http://localhost:8970/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=valheim
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
