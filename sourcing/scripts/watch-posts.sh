#!/usr/bin/env bash
# Watches the clipboard and saves each new copy as its own HTML file,
# ready for the extractor's `merge` command. Runs until you press Ctrl+C.
#
# Usage:
#   1. Run this script once: ./scripts/watch-posts.sh [output-dir]
#      Defaults to raw/<today's date> if no directory is given.
#   2. Copy each post's DOM in Chrome DevTools ("Copy element" / "Copy
#      outerHTML") — every new copy is detected and saved automatically.
#   3. Press Ctrl+C when done.
set -euo pipefail

OUTPUT_DIR="${1:-raw/$(date -u +%Y-%m-%d)}"
mkdir -p "$OUTPUT_DIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

COUNT=0
LAST=""

echo "Watching clipboard — copy posts now. Press Ctrl+C to stop."
echo "Saving to: $OUTPUT_DIR"

trap 'echo; echo "Stopped. Saved $COUNT post(s) to $OUTPUT_DIR"; exit 0' INT

while true; do
    CURRENT="$(pbpaste 2>/dev/null || true)"

    if [ -n "$CURRENT" ] && [ "$CURRENT" != "$LAST" ]; then
        if OUTPUT="$("$SCRIPT_DIR/save-post.sh" "$OUTPUT_DIR" 2>&1)"; then
            COUNT=$((COUNT + 1))
            echo "[$COUNT] $OUTPUT"
        else
            echo "$OUTPUT" >&2
        fi
        LAST="$CURRENT"
    fi

    sleep 0.5
done
