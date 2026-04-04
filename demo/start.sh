#!/bin/bash
set -e

cd /opt/stockyard-demo

# Start tools in background
for tool_port in "bounty:9320" "corral:8760" "seismograph:9680" "paddock:8750" "saltlick:8730"; do
    tool="${tool_port%%:*}"
    port="${tool_port##*:}"
    mkdir -p "data/$tool"
    PORT=$port DATA_DIR="data/$tool" ./bin/stockyard-$tool > "logs/$tool.log" 2>&1 &
    echo "Started $tool on :$port"
done

# Wait for tools
sleep 4

# Seed data (only if databases are empty)
if [ ! -f "data/.seeded" ]; then
    python3 seed.py
    touch "data/.seeded"
fi

# Run Hub in foreground (so Docker keeps running)
mkdir -p data/hub
echo "Starting Hub on :${PORT:-9800}"
PORT=${PORT:-9800} DATA_DIR=data/hub BIN_DIR=bin DEMO_MODE=true exec ./bin/stockyard-hub
