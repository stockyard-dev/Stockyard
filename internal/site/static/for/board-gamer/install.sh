#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Board Game Groups"
echo "  9 tools — self-hosted on your hardware"
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

BUNDLE_DIR="$HOME/stockyard-board-gamer"
mkdir -p "$BUNDLE_DIR/tools" "$BUNDLE_DIR/data"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

FAILED=0

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
echo "  Downloading Collection..."
URL="https://github.com/stockyard-dev/stockyard-collection/releases/latest/download/stockyard-collection_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-collection_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-collection" 2>/dev/null || \
  mv "$TMP/stockyard-collection" "$BUNDLE_DIR/tools/stockyard-collection" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-collection" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Collection"
else
  echo "    ✗ Collection (failed)"
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
echo "  Downloading Tournament..."
URL="https://github.com/stockyard-dev/stockyard-tournament/releases/latest/download/stockyard-tournament_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-tournament_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-tournament" 2>/dev/null || \
  mv "$TMP/stockyard-tournament" "$BUNDLE_DIR/tools/stockyard-tournament" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-tournament" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Tournament"
else
  echo "    ✗ Tournament (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Board Game Groups..."
echo ""
PORT=10230 "$DIR/tools/stockyard-quartermaster" -port 10230 -data "$DATA" >/dev/null 2>&1 &
PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA" >/dev/null 2>&1 &
PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA" >/dev/null 2>&1 &
PORT=8640 "$DIR/tools/stockyard-tally" -port 8640 -data "$DATA" >/dev/null 2>&1 &
PORT=9370 "$DIR/tools/stockyard-notebook" -port 9370 -data "$DATA" >/dev/null 2>&1 &
PORT=10070 "$DIR/tools/stockyard-agora" -port 10070 -data "$DATA" >/dev/null 2>&1 &
PORT=9100 "$DIR/tools/stockyard-collection" -port 9100 -data "$DATA" >/dev/null 2>&1 &
PORT=9100 "$DIR/tools/stockyard-checkout" -port 9100 -data "$DATA" >/dev/null 2>&1 &
PORT=9804 "$DIR/tools/stockyard-tournament" -port 9804 -data "$DATA" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Quartermaster             http://localhost:10230/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Tally                     http://localhost:8640/ui"
echo "  ✓ Notebook                  http://localhost:9370/ui"
echo "  ✓ Agora                     http://localhost:10070/ui"
echo "  ✓ Collection                http://localhost:9100/ui"
echo "  ✓ Checkout                  http://localhost:9100/ui"
echo "  ✓ Tournament                http://localhost:9804/ui"
echo ""
echo "  All tools running. Press Ctrl+C to stop."
echo ""
if command -v xdg-open &>/dev/null; then
  xdg-open "http://localhost:10230/ui" 2>/dev/null &
elif command -v open &>/dev/null; then
  open "http://localhost:10230/ui" 2>/dev/null &
fi
wait
STARTEOF
chmod +x "$BUNDLE_DIR/start.sh"

cat > "$BUNDLE_DIR/stop.sh" << 'STOPEOF'
#!/bin/bash
echo "  Stopping Stockyard tools..."
pkill -f "stockyard-quartermaster" 2>/dev/null && echo "  ✓ Stopped Quartermaster" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-tally" 2>/dev/null && echo "  ✓ Stopped Tally" || true
pkill -f "stockyard-notebook" 2>/dev/null && echo "  ✓ Stopped Notebook" || true
pkill -f "stockyard-agora" 2>/dev/null && echo "  ✓ Stopped Agora" || true
pkill -f "stockyard-collection" 2>/dev/null && echo "  ✓ Stopped Collection" || true
pkill -f "stockyard-checkout" 2>/dev/null && echo "  ✓ Stopped Checkout" || true
pkill -f "stockyard-tournament" 2>/dev/null && echo "  ✓ Stopped Tournament" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR BOARD GAME GROUPS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Quartermaster             http://localhost:10230/ui
  Roster                    http://localhost:8970/ui
  Sundial                   http://localhost:9890/ui
  Tally                     http://localhost:8640/ui
  Notebook                  http://localhost:9370/ui
  Agora                     http://localhost:10070/ui
  Collection                http://localhost:9100/ui
  Checkout                  http://localhost:9100/ui
  Tournament                http://localhost:9804/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=board-gamer
Help:    hello@stockyard.dev
READMEEOF

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 9 tools installed to $BUNDLE_DIR/"
else
  echo "  ⚠ $FAILED tool(s) failed. The rest are ready."
fi
echo ""
echo "  Next steps:"
echo "    cd $BUNDLE_DIR"
echo "    ./start.sh"
echo ""
