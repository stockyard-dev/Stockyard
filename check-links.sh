#!/usr/bin/env bash

# check-links.sh — Scan exported site for broken internal links.

DIST="${1:-dist}"

if [ ! -d "$DIST" ]; then
    echo "Error: $DIST/ not found. Run export-site.sh first."
    exit 1
fi

echo "Link Checker: scanning $DIST/"
echo "================================"

BROKEN_LOG=$(mktemp)

# Collect all internal links
for htmlfile in $(find "$DIST" -name '*.html' -type f); do
    # Extract href="..." and src="..." values
    for link in $(grep -oP '(?:href|src)="\K[^"]+' "$htmlfile" 2>/dev/null || true); do
        # Skip external, anchors, javascript
        case "$link" in
            http://*|https://*|mailto:*|javascript:*|//*|\#*|data:*) continue ;;
        esac

        # Resolve path
        if [ "${link:0:1}" = "/" ]; then
            TARGET="$DIST$link"
        else
            TARGET="$(dirname "$htmlfile")/$link"
        fi

        # Strip fragment and query
        TARGET="${TARGET%%#*}"
        TARGET="${TARGET%%\?*}"
        [ -z "$TARGET" ] && continue

        # Check existence
        if [ -f "$TARGET" ]; then
            continue
        elif [ -d "$TARGET" ] && [ -f "$TARGET/index.html" ]; then
            continue
        elif [ "${TARGET: -1}" = "/" ] && [ -f "${TARGET}index.html" ]; then
            continue
        else
            REL="${htmlfile#$DIST/}"
            echo "  BROKEN  $REL -> $link"
            echo "$link" >> "$BROKEN_LOG"
        fi
    done
done

echo ""
echo "================================"
if [ -s "$BROKEN_LOG" ]; then
    COUNT=$(wc -l < "$BROKEN_LOG")
    echo "$COUNT broken link(s) found."
    echo ""
    echo "Summary:"
    sort "$BROKEN_LOG" | uniq -c | sort -rn | head -10
    rm -f "$BROKEN_LOG"
    exit 1
else
    echo "All internal links resolve."
    rm -f "$BROKEN_LOG"
    exit 0
fi
