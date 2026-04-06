#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for YouTubers"
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

BUNDLE_DIR="$HOME/stockyard-youtube"
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
echo "  Downloading Roundup..."
URL="https://github.com/stockyard-dev/stockyard-roundup/releases/latest/download/stockyard-roundup_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-roundup_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-roundup" 2>/dev/null || \
  mv "$TMP/stockyard-roundup" "$BUNDLE_DIR/tools/stockyard-roundup" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-roundup" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Roundup"
else
  echo "    ✗ Roundup (failed)"
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

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for YouTubers..."
echo ""
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
mkdir -p "$DATA/billfold" && PORT=9070 "$DIR/tools/stockyard-billfold" -port 9070 -data "$DATA/billfold" >/dev/null 2>&1 &
mkdir -p "$DATA/steward" && PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA/steward" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/roundup" && PORT=8700 "$DIR/tools/stockyard-roundup" -port 8700 -data "$DATA/roundup" >/dev/null 2>&1 &
mkdir -p "$DATA/notebook" && PORT=9370 "$DIR/tools/stockyard-notebook" -port 9370 -data "$DATA/notebook" >/dev/null 2>&1 &
mkdir -p "$DATA/tally" && PORT=8640 "$DIR/tools/stockyard-tally" -port 8640 -data "$DATA/tally" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Billfold                  http://localhost:9070/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Roundup                   http://localhost:8700/ui"
echo "  ✓ Notebook                  http://localhost:9370/ui"
echo "  ✓ Tally                     http://localhost:8640/ui"
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
pkill -f "stockyard-billfold" 2>/dev/null && echo "  ✓ Stopped Billfold" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-roundup" 2>/dev/null && echo "  ✓ Stopped Roundup" || true
pkill -f "stockyard-notebook" 2>/dev/null && echo "  ✓ Stopped Notebook" || true
pkill -f "stockyard-tally" 2>/dev/null && echo "  ✓ Stopped Tally" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR YOUTUBERS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Billfold                  http://localhost:9070/ui
  Steward                   http://localhost:9840/ui
  Sundial                   http://localhost:9890/ui
  Roundup                   http://localhost:8700/ui
  Notebook                  http://localhost:9370/ui
  Tally                     http://localhost:8640/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=youtube
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
