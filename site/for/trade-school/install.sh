#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Trade Schools & Vocational Programs"
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

BUNDLE_DIR="$HOME/stockyard-trade-school"
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
echo "  Downloading Trailhead..."
URL="https://github.com/stockyard-dev/stockyard-trailhead/releases/latest/download/stockyard-trailhead_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-trailhead_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-trailhead" 2>/dev/null || \
  mv "$TMP/stockyard-trailhead" "$BUNDLE_DIR/tools/stockyard-trailhead" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-trailhead" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Trailhead"
else
  echo "    ✗ Trailhead (failed)"
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

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Trade Schools & Vocational Programs..."
echo ""
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
mkdir -p "$DATA/billfold" && PORT=9070 "$DIR/tools/stockyard-billfold" -port 9070 -data "$DATA/billfold" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/roster" && PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA/roster" >/dev/null 2>&1 &
mkdir -p "$DATA/trailhead" && PORT=9380 "$DIR/tools/stockyard-trailhead" -port 9380 -data "$DATA/trailhead" >/dev/null 2>&1 &
mkdir -p "$DATA/steward" && PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA/steward" >/dev/null 2>&1 &
mkdir -p "$DATA/surveyor" && PORT=9290 "$DIR/tools/stockyard-surveyor" -port 9290 -data "$DATA/surveyor" >/dev/null 2>&1 &
mkdir -p "$DATA/deposition" && PORT=9310 "$DIR/tools/stockyard-deposition" -port 9310 -data "$DATA/deposition" >/dev/null 2>&1 &
mkdir -p "$DATA/curriculum" && PORT=9813 "$DIR/tools/stockyard-curriculum" -port 9813 -data "$DATA/curriculum" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Billfold                  http://localhost:9070/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo "  ✓ Trailhead                 http://localhost:9380/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo "  ✓ Surveyor                  http://localhost:9290/ui"
echo "  ✓ Deposition                http://localhost:9310/ui"
echo "  ✓ Curriculum                http://localhost:9813/ui"
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
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
pkill -f "stockyard-trailhead" 2>/dev/null && echo "  ✓ Stopped Trailhead" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
pkill -f "stockyard-surveyor" 2>/dev/null && echo "  ✓ Stopped Surveyor" || true
pkill -f "stockyard-deposition" 2>/dev/null && echo "  ✓ Stopped Deposition" || true
pkill -f "stockyard-curriculum" 2>/dev/null && echo "  ✓ Stopped Curriculum" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR TRADE SCHOOLS & VOCATIONAL PROGRAMS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Billfold                  http://localhost:9070/ui
  Sundial                   http://localhost:9890/ui
  Roster                    http://localhost:8970/ui
  Trailhead                 http://localhost:9380/ui
  Steward                   http://localhost:9840/ui
  Surveyor                  http://localhost:9290/ui
  Deposition                http://localhost:9310/ui
  Curriculum                http://localhost:9813/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=trade-school
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
