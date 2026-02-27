#!/bin/bash
# Build the dashboard by concatenating component files into static/index.html.
# Run from the repo root: ./internal/dashboard/build.sh
#
# Source files in src/ are split by component for maintainability.
# The output is a single index.html embedded into the Go binary via //go:embed.

set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"

SRC="$DIR/src"
OUT="$DIR/static/index.html"

# Concatenate in order: head.html, JS files (sorted), tail.html
cat \
  "$SRC/head.html" \
  "$SRC/00-utils.js" \
  "$SRC/01-components.js" \
  "$SRC/02-overview.js" \
  "$SRC/03-proxy.js" \
  "$SRC/04-charts.js" \
  "$SRC/05-observe.js" \
  "$SRC/06-trust.js" \
  "$SRC/07-studio.js" \
  "$SRC/08-forge.js" \
  "$SRC/09-exchange.js" \
  "$SRC/10-settings.js" \
  "$SRC/11-app.js" \
  "$SRC/tail.html" \
  > "$OUT"

LINES=$(wc -l < "$OUT")
echo "dashboard: built $OUT ($LINES lines from $(ls "$SRC"/*.js | wc -l) components)"
