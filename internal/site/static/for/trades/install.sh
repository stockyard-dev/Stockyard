#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "  Stockyard for Plumbers, Electricians & HVAC"
echo "  11 tools — self-hosted on your hardware"
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

BUNDLE_DIR="$HOME/stockyard-trades"
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
echo "  Downloading Estimate..."
URL="https://github.com/stockyard-dev/stockyard-estimate/releases/latest/download/stockyard-estimate_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-estimate_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-estimate" 2>/dev/null || \
  mv "$TMP/stockyard-estimate" "$BUNDLE_DIR/tools/stockyard-estimate" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-estimate" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Estimate"
else
  echo "    ✗ Estimate (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Fleet..."
URL="https://github.com/stockyard-dev/stockyard-fleet/releases/latest/download/stockyard-fleet_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-fleet_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-fleet" 2>/dev/null || \
  mv "$TMP/stockyard-fleet" "$BUNDLE_DIR/tools/stockyard-fleet" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-fleet" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Fleet"
else
  echo "    ✗ Fleet (failed)"
  FAILED=$((FAILED + 1))
fi
echo "  Downloading Permit..."
URL="https://github.com/stockyard-dev/stockyard-permit/releases/latest/download/stockyard-permit_${OS}_${ARCH}.tar.gz"
if curl -fsSL "$URL" -o "$TMP/archive.tar.gz" 2>/dev/null; then
  tar -xzf "$TMP/archive.tar.gz" -C "$TMP" 2>/dev/null
  mv "$TMP/stockyard-permit_${OS}_${ARCH}" "$BUNDLE_DIR/tools/stockyard-permit" 2>/dev/null || \
  mv "$TMP/stockyard-permit" "$BUNDLE_DIR/tools/stockyard-permit" 2>/dev/null || true
  chmod +x "$BUNDLE_DIR/tools/stockyard-permit" 2>/dev/null
  rm -f "$TMP/archive.tar.gz"
  echo "    ✓ Permit"
else
  echo "    ✗ Permit (failed)"
  FAILED=$((FAILED + 1))
fi

cat > "$BUNDLE_DIR/start.sh" << 'STARTEOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
DATA="$DIR/data"
mkdir -p "$DATA"
echo ""
echo "  Starting Stockyard for Plumbers, Electricians & HVAC..."
echo ""
PORT=9080 "$DIR/tools/stockyard-dossier" -port 9080 -data "$DATA" >/dev/null 2>&1 &
PORT=9070 "$DIR/tools/stockyard-billfold" -port 9070 -data "$DATA" >/dev/null 2>&1 &
PORT=8700 "$DIR/tools/stockyard-roundup" -port 8700 -data "$DATA" >/dev/null 2>&1 &
PORT=9890 "$DIR/tools/stockyard-sundial" -port 9890 -data "$DATA" >/dev/null 2>&1 &
PORT=9840 "$DIR/tools/stockyard-steward" -port 9840 -data "$DATA" >/dev/null 2>&1 &
PORT=10230 "$DIR/tools/stockyard-quartermaster" -port 10230 -data "$DATA" >/dev/null 2>&1 &
PORT=9310 "$DIR/tools/stockyard-deposition" -port 9310 -data "$DATA" >/dev/null 2>&1 &
PORT=9290 "$DIR/tools/stockyard-surveyor" -port 9290 -data "$DATA" >/dev/null 2>&1 &
PORT=9802 "$DIR/tools/stockyard-estimate" -port 9802 -data "$DATA" >/dev/null 2>&1 &
PORT=9809 "$DIR/tools/stockyard-fleet" -port 9809 -data "$DATA" >/dev/null 2>&1 &
PORT=9811 "$DIR/tools/stockyard-permit" -port 9811 -data "$DATA" >/dev/null 2>&1 &
sleep 1
echo ""
echo "  ✓ Dossier                   http://localhost:9080/ui"
echo "  ✓ Billfold                  http://localhost:9070/ui"
echo "  ✓ Roundup                   http://localhost:8700/ui"
echo "  ✓ Sundial                   http://localhost:9890/ui"
echo "  ✓ Steward                   http://localhost:9840/ui"
echo "  ✓ Quartermaster             http://localhost:10230/ui"
echo "  ✓ Deposition                http://localhost:9310/ui"
echo "  ✓ Surveyor                  http://localhost:9290/ui"
echo "  ✓ Estimate                  http://localhost:9802/ui"
echo "  ✓ Fleet                     http://localhost:9809/ui"
echo "  ✓ Permit                    http://localhost:9811/ui"
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
pkill -f "stockyard-roundup" 2>/dev/null && echo "  ✓ Stopped Roundup" || true
pkill -f "stockyard-sundial" 2>/dev/null && echo "  ✓ Stopped Sundial" || true
pkill -f "stockyard-steward" 2>/dev/null && echo "  ✓ Stopped Steward" || true
pkill -f "stockyard-quartermaster" 2>/dev/null && echo "  ✓ Stopped Quartermaster" || true
pkill -f "stockyard-deposition" 2>/dev/null && echo "  ✓ Stopped Deposition" || true
pkill -f "stockyard-surveyor" 2>/dev/null && echo "  ✓ Stopped Surveyor" || true
pkill -f "stockyard-estimate" 2>/dev/null && echo "  ✓ Stopped Estimate" || true
pkill -f "stockyard-fleet" 2>/dev/null && echo "  ✓ Stopped Fleet" || true
pkill -f "stockyard-permit" 2>/dev/null && echo "  ✓ Stopped Permit" || true
echo "  Done."
STOPEOF
chmod +x "$BUNDLE_DIR/stop.sh"

cat > "$BUNDLE_DIR/README.txt" << 'READMEEOF'
STOCKYARD FOR PLUMBERS, ELECTRICIANS & HVAC

Start:   ./start.sh
Stop:    ./stop.sh
Data:    ./data/

Tools:
  Dossier                   http://localhost:9080/ui
  Billfold                  http://localhost:9070/ui
  Roundup                   http://localhost:8700/ui
  Sundial                   http://localhost:9890/ui
  Steward                   http://localhost:9840/ui
  Quartermaster             http://localhost:10230/ui
  Deposition                http://localhost:9310/ui
  Surveyor                  http://localhost:9290/ui
  Estimate                  http://localhost:9802/ui
  Fleet                     http://localhost:9809/ui
  Permit                    http://localhost:9811/ui

License: export STOCKYARD_LICENSE_KEY=your_key
Trial:   https://stockyard.dev/pricing/?bundle=trades
Help:    hello@stockyard.dev
READMEEOF

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "  ✓ All 11 tools installed to $BUNDLE_DIR/"
else
  echo "  ⚠ $FAILED tool(s) failed. The rest are ready."
fi
echo ""
echo "  Next steps:"
echo "    cd $BUNDLE_DIR"
echo "    ./start.sh"
echo ""
