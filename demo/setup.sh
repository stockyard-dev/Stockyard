#!/bin/bash
set -e

# Stockyard Demo Instance Setup
# Runs Hub + 5 tools with realistic seeded data
# Deploy: run this on a $5 VPS, point demo.stockyard.dev at it

DEMO_DIR="${DEMO_DIR:-/opt/stockyard-demo}"
GITHUB_ORG="stockyard-dev"
TOOLS="hub bounty corral seismograph paddock saltlick"

echo ""
echo "  ┌─────────────────────────────────────┐"
echo "  │  Stockyard Demo Setup               │"
echo "  │  Hub + 5 tools with sample data     │"
echo "  └─────────────────────────────────────┘"
echo ""

# Detect OS/arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac
ASSET_SUFFIX="${OS}_${ARCH}"
echo "  Platform: ${OS}/${ARCH}"

# Create directories
mkdir -p "$DEMO_DIR"/{bin,data,logs}
cd "$DEMO_DIR"

# Download each tool
echo "  Downloading tools..."
for tool in $TOOLS; do
    echo -n "    $tool... "
    REPO="${GITHUB_ORG}/stockyard-${tool}"
    RELEASE_URL=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep "browser_download_url.*${ASSET_SUFFIX}" | head -1 | cut -d'"' -f4)
    if [ -z "$RELEASE_URL" ]; then
        echo "SKIP (no binary for ${ASSET_SUFFIX})"
        continue
    fi
    curl -fsSL "$RELEASE_URL" -o "/tmp/stockyard-${tool}.tar.gz"
    tar xzf "/tmp/stockyard-${tool}.tar.gz" -C "$DEMO_DIR/bin/"
    # Rename extracted binary to clean name
    mv "$DEMO_DIR/bin/stockyard-${tool}"* "$DEMO_DIR/bin/stockyard-${tool}" 2>/dev/null || true
    chmod +x "$DEMO_DIR/bin/stockyard-${tool}"
    rm -f "/tmp/stockyard-${tool}.tar.gz"
    echo "✓"
done

# Port assignments
declare -A PORTS
PORTS[hub]=9800
PORTS[bounty]=9320
PORTS[corral]=8760
PORTS[seismograph]=9680
PORTS[paddock]=8750
PORTS[saltlick]=8730

# Start tools (not Hub yet)
echo ""
echo "  Starting tools..."
for tool in bounty corral seismograph paddock saltlick; do
    PORT=${PORTS[$tool]}
    DATA="$DEMO_DIR/data/$tool"
    LOG="$DEMO_DIR/logs/$tool.log"
    mkdir -p "$DATA"
    
    PORT=$PORT DATA_DIR="$DATA" nohup "$DEMO_DIR/bin/stockyard-$tool" > "$LOG" 2>&1 &
    echo "    $tool on :$PORT (PID $!)"
done

# Wait for tools to start
echo "  Waiting for tools to start..."
sleep 4

# Seed demo data
echo ""
if command -v python3 &> /dev/null; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    python3 "$SCRIPT_DIR/seed.py"
else
    echo "  WARN: python3 not found, skipping seed (install python3 and run seed.py manually)"
fi

# Start Hub
echo ""
echo "  Starting Hub..."
HUB_DATA="$DEMO_DIR/data/hub"
mkdir -p "$HUB_DATA"
PORT=${PORTS[hub]} DATA_DIR="$HUB_DATA" BIN_DIR="$DEMO_DIR/bin" \
    DEMO_MODE=true \
    nohup "$DEMO_DIR/bin/stockyard-hub" > "$DEMO_DIR/logs/hub.log" 2>&1 &
echo "    Hub on :${PORTS[hub]} (PID $!)"

sleep 2
echo ""
echo "  ✓ Demo instance running"
echo ""
echo "  Hub:          http://localhost:${PORTS[hub]}/ui"
echo "  Bounty:       http://localhost:${PORTS[bounty]}/ui"
echo "  Corral:       http://localhost:${PORTS[corral]}/ui"
echo "  Seismograph:  http://localhost:${PORTS[seismograph]}/ui"
echo "  Paddock:      http://localhost:${PORTS[paddock]}/ui"
echo "  Salt Lick:    http://localhost:${PORTS[saltlick]}/ui"
echo ""
echo "  To stop: pkill -f 'stockyard-'"
echo "  Logs:    $DEMO_DIR/logs/"
echo ""
