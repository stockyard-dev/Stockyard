#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Preschools & Montessori Schools"
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

BUNDLE_DIR="$HOME/stockyard-preschool"
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
echo "  Downloading Dispatch..."
URL="https://github.com/stockyard-dev/stockyard-dispatch/releases/latest/download/stockyard-dispatch_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-dispatch_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-dispatch" 2>/dev/null || \
  mv "$TMP/stockyard-dispatch" "$BUNDLE_DIR/tools/stockyard-dispatch" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-dispatch" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Dispatch"
else
  echo "    ✗ Dispatch (failed)"
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
echo "  Downloading Curriculum..."
URL="https://github.com/stockyard-dev/stockyard-curriculum/releases/latest/download/stockyard-curriculum_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-curriculum_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-curriculum" 2>/dev/null || \
  mv "$TMP/stockyard-curriculum" "$BUNDLE_DIR/tools/stockyard-curriculum" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-curriculum" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Curriculum"
else
  echo "    ✗ Curriculum (failed)"
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
echo "  Downloading Waiver..."
URL="https://github.com/stockyard-dev/stockyard-waiver/releases/latest/download/stockyard-waiver_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-waiver_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-waiver" 2>/dev/null || \
  mv "$TMP/stockyard-waiver" "$BUNDLE_DIR/tools/stockyard-waiver" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-waiver" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Waiver"
else
  echo "    ✗ Waiver (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Preschools & Montessori Schools..."
echo ""
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
mkdir -p "$DATA/checkin" && PORT=9807 "$DIR/tools/stockyard-checkin" -port 9807 -data "$DATA/checkin" >/dev/null 2>&1 &
mkdir -p "$DATA/dispatch" && PORT=8560 "$DIR/tools/stockyard-dispatch" -port 8560 -data "$DATA/dispatch" >/dev/null 2>&1 &
mkdir -p "$DATA/billfold" && PORT=9070 "$DIR/tools/stockyard-billfold" -port 9070 -data "$DATA/billfold" >/dev/null 2>&1 &
mkdir -p "$DATA/curriculum" && PORT=9813 "$DIR/tools/stockyard-curriculum" -port 9813 -data "$DATA/curriculum" >/dev/null 2>&1 &
mkdir -p "$DATA/surveyor" && PORT=9290 "$DIR/tools/stockyard-surveyor" -port 9290 -data "$DATA/surveyor" >/dev/null 2>&1 &
mkdir -p "$DATA/waiver" && PORT=9801 "$DIR/tools/stockyard-waiver" -port 9801 -data "$DATA/waiver" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Checkin                   http://localhost:9807/ui"
echo "  ✓ Dispatch                  http://localhost:8560/ui"
echo "  ✓ Billfold                  http://localhost:9070/ui"
echo "  ✓ Curriculum                http://localhost:9813/ui"
echo "  ✓ Surveyor                  http://localhost:9290/ui"
echo "  ✓ Waiver                    http://localhost:9801/ui"
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
pkill -f "stockyard-dispatch" 2>/dev/null && echo "  ✓ Stopped Dispatch" || true
pkill -f "stockyard-billfold" 2>/dev/null && echo "  ✓ Stopped Billfold" || true
pkill -f "stockyard-curriculum" 2>/dev/null && echo "  ✓ Stopped Curriculum" || true
pkill -f "stockyard-surveyor" 2>/dev/null && echo "  ✓ Stopped Surveyor" || true
pkill -f "stockyard-waiver" 2>/dev/null && echo "  ✓ Stopped Waiver" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR PRESCHOOLS & MONTESSORI SCHOOLS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Checkin                   http://localhost:9807/ui
  Dispatch                  http://localhost:8560/ui
  Billfold                  http://localhost:9070/ui
  Curriculum                http://localhost:9813/ui
  Surveyor                  http://localhost:9290/ui
  Waiver                    http://localhost:9801/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=preschool
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
