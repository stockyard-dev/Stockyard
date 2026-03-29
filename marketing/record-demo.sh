#!/bin/bash
# Record a terminal demo for Product Hunt
# Prerequisites: asciinema, agg (https://github.com/asciinema/agg)
#
# Install:
#   brew install asciinema
#   cargo install --git https://github.com/asciinema/agg
#
# Usage:
#   bash record-demo.sh
#   # This creates demo.gif (~2MB)
#
# Then upload to PH as the first gallery item.

set -e

RECORDING="demo.cast"
GIF="demo.gif"

echo "Starting recording in 3 seconds..."
echo "Follow the script below — type naturally, don't rush."
echo ""
echo "═══ SCRIPT ═══"
echo "1. curl -sSL stockyard.dev/install.sh | sh"
echo "2. stockyard"
echo "3. (wait for boot message)"
echo "4. (in new terminal) curl localhost:7749/v1/chat/completions \\"
echo "     -H 'Content-Type: application/json' \\"
echo "     -H 'Authorization: Bearer \$OPENAI_API_KEY' \\"
echo "     -d '{\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}'"
echo "5. curl localhost:7749/api/observe/traces?limit=1 | jq ."
echo "═══════════════"
echo ""
sleep 3

# Record
asciinema rec "$RECORDING" \
  --title "Stockyard — Install to first request in 60 seconds" \
  --cols 100 \
  --rows 30 \
  --idle-time-limit 2

echo ""
echo "Recording saved to $RECORDING"
echo ""

# Convert to GIF
if command -v agg >/dev/null 2>&1; then
  echo "Converting to GIF..."
  agg "$RECORDING" "$GIF" \
    --font-family "JetBrains Mono" \
    --theme "monokai" \
    --cols 100 \
    --rows 30 \
    --speed 1.5
  echo "GIF saved to $GIF ($(du -h $GIF | cut -f1))"
  echo ""
  echo "Upload $GIF as the first gallery image on Product Hunt."
else
  echo "Install agg to convert to GIF:"
  echo "  cargo install --git https://github.com/asciinema/agg"
  echo ""
  echo "Or upload the .cast to asciinema.org and link it."
fi
