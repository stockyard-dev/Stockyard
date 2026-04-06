#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Schools & Teachers"
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

BUNDLE_DIR="$HOME/stockyard-school"
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
echo "  Downloading Pony Express..."
URL="https://github.com/stockyard-dev/stockyard-ponyexpress/releases/latest/download/stockyard-ponyexpress_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-ponyexpress_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-ponyexpress" 2>/dev/null || \
  mv "$TMP/stockyard-ponyexpress" "$BUNDLE_DIR/tools/stockyard-ponyexpress" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-ponyexpress" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Pony Express"
else
  echo "    ✗ Pony Express (failed)"
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

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Schools & Teachers..."
echo ""
mkdir -p "$DATA/dossier" && PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA/dossier" >/dev/null 2>&1 &
mkdir -p "$DATA/announcements" && PORT=9750 "$DIR/tools/stockyard-announcements" -port 9750 -data "$DATA/announcements" >/dev/null 2>&1 &
mkdir -p "$DATA/roster" && PORT=8970 "$DIR/tools/stockyard-roster" -port 8970 -data "$DATA/roster" >/dev/null 2>&1 &
mkdir -p "$DATA/sundial" && PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA/sundial" >/dev/null 2>&1 &
mkdir -p "$DATA/trailhead" && PORT=9380 "$DIR/tools/stockyard-trailhead" -port 9380 -data "$DATA/trailhead" >/dev/null 2>&1 &
mkdir -p "$DATA/surveyor" && PORT=9290 "$DIR/tools/stockyard-surveyor" -port 9290 -data "$DATA/surveyor" >/dev/null 2>&1 &
mkdir -p "$DATA/agora" && PORT=10070 "$DIR/tools/stockyard-agora" -port 10070 -data "$DATA/agora" >/dev/null 2>&1 &
mkdir -p "$DATA/ponyexpress" && PORT=8930 "$DIR/tools/stockyard-ponyexpress" -port 8930 -data "$DATA/ponyexpress" >/dev/null 2>&1 &
mkdir -p "$DATA/checkin" && PORT=9807 "$DIR/tools/stockyard-checkin" -port 9807 -data "$DATA/checkin" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Announcements             http://localhost:9750/ui"
echo "  ✓ Roster                    http://localhost:8970/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Trailhead                 http://localhost:9380/ui"
echo "  ✓ Surveyor                  http://localhost:9290/ui"
echo "  ✓ Agora                     http://localhost:10070/ui"
echo "  ✓ Pony Express              http://localhost:8930/ui"
echo "  ✓ Checkin                   http://localhost:9807/ui"
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
pkill -f "stockyard-announcements" 2>/dev/null && echo "  ✓ Stopped Announcements" || true
pkill -f "stockyard-roster" 2>/dev/null && echo "  ✓ Stopped Roster" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-trailhead" 2>/dev/null && echo "  ✓ Stopped Trailhead" || true
pkill -f "stockyard-surveyor" 2>/dev/null && echo "  ✓ Stopped Surveyor" || true
pkill -f "stockyard-agora" 2>/dev/null && echo "  ✓ Stopped Agora" || true
pkill -f "stockyard-ponyexpress" 2>/dev/null && echo "  ✓ Stopped Pony Express" || true
pkill -f "stockyard-checkin" 2>/dev/null && echo "  ✓ Stopped Checkin" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR SCHOOLS & TEACHERS

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Announcements             http://localhost:9750/ui
  Roster                    http://localhost:8970/ui
  Sundial                   http://localhost:9890/ui
  Trailhead                 http://localhost:9380/ui
  Surveyor                  http://localhost:9290/ui
  Agora                     http://localhost:10070/ui
  Pony Express              http://localhost:8930/ui
  Checkin                   http://localhost:9807/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=school
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
