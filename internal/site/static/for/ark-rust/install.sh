#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for ARK / Rust Server Admins"
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

BUNDLE_DIR="$HOME/stockyard-ark-rust"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

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
echo "  Downloading Campfire..."
URL="https://github.com/stockyard-dev/stockyard-campfire/releases/latest/download/stockyard-campfire_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-campfire_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-campfire" 2>/dev/null || \
  mv "$TMP/stockyard-campfire" "$BUNDLE_DIR/tools/stockyard-campfire" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-campfire" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Campfire"
else
  echo "    ✗ Campfire (failed)"
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
echo "  Downloading Deposition..."
URL="https://github.com/stockyard-dev/stockyard-deposition/releases/latest/download/stockyard-deposition_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-deposition_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-deposition" 2>/dev/null || \
  mv "$TMP/stockyard-deposition" "$BUNDLE_DIR/tools/stockyard-deposition" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-deposition" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Deposition"
else
  echo "    ✗ Deposition (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for ARK / Rust Server Admins..."
echo ""
mkdir -p "$DATA/headcount" && PORT=8690 "$DIR/tools/stockyard-headcount" -port 8690 -data "$DATA/headcount" >/dev/null 2>&1 &
mkdir -p "$DATA/sentinel" && PORT=8680 "$DIR/tools/stockyard-sentinel" -port 8680 -data "$DATA/sentinel" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/campfire" && PORT=8850 "$DIR/tools/stockyard-campfire" -port 8850 -data "$DATA/campfire" >/dev/null 2>&1 &
mkdir -p "$DATA/roster" && PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA/roster" >/dev/null 2>&1 &
mkdir -p "$DATA/outpost" && PORT=8740 "$DIR/tools/stockyard-outpost" -port 8740 -data "$DATA/outpost" >/dev/null 2>&1 &
mkdir -p "$DATA/announcements" && PORT=9750 "$DIR/tools/stockyard-announcements" -port 9750 -data "$DATA/announcements" >/dev/null 2>&1 &
mkdir -p "$DATA/deposition" && PORT=9310 "$DIR/tools/stockyard-deposition" -port 9310 -data "$DATA/deposition" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Headcount                 http://localhost:8690/ui"
echo "  ✓ Sentinel                  http://localhost:8680/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Campfire                  http://localhost:8850/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo "  ✓ Outpost                   http://localhost:8740/ui"
echo "  ✓ Announcements             http://localhost:9750/ui"
echo "  ✓ Deposition                http://localhost:9310/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:8690/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:8690/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-headcount" 2>/dev/null && echo "  ✓ Stopped Headcount" || true
pkill -f "stockyard-sentinel" 2>/dev/null && echo "  ✓ Stopped Sentinel" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-campfire" 2>/dev/null && echo "  ✓ Stopped Campfire" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
pkill -f "stockyard-outpost" 2>/dev/null && echo "  ✓ Stopped Outpost" || true
pkill -f "stockyard-announcements" 2>/dev/null && echo "  ✓ Stopped Announcements" || true
pkill -f "stockyard-deposition" 2>/dev/null && echo "  ✓ Stopped Deposition" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR ARK / RUST SERVER ADMINS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Headcount                 http://localhost:8690/ui
  Sentinel                  http://localhost:8680/ui
  Sundial                   http://localhost:9890/ui
  Campfire                  http://localhost:8850/ui
  Roster                    http://localhost:8970/ui
  Outpost                   http://localhost:8740/ui
  Announcements             http://localhost:9750/ui
  Deposition                http://localhost:9310/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=ark-rust
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
