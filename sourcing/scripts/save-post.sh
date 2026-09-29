#!/usr/bin/env bash
# Saves the current clipboard content as a new HTML file, ready for the
# extractor's `merge` command to pick up.
#
# Usage:
#   1. Copy a post's DOM in Chrome DevTools ("Copy element" / "Copy outerHTML").
#   2. Run this script: ./scripts/save-post.sh [output-dir]
#      Defaults to raw/<today's date> if no directory is given.
#   3. Repeat for each post.
set -euo pipefail

CONTENT="$(pbpaste)"

if [ -z "$CONTENT" ]; then
    echo "Error: clipboard is empty. Copy the post's DOM first." >&2
    exit 1
fi

if [[ "$CONTENT" != *"<"* ]]; then
    echo "Warning: clipboard content doesn't look like HTML — saving anyway." >&2
fi

OUTPUT_DIR="${1:-raw/$(date -u +%Y-%m-%d)}"
mkdir -p "$OUTPUT_DIR"

BASENAME="post-$(date -u +%Y-%m-%dT%H-%M-%S)"
FILE="$OUTPUT_DIR/$BASENAME.html"

# Guard against two saves landing in the same second (unlikely for a manual
# workflow, but cheap to handle): append a numeric suffix instead of
# silently overwriting.
SUFFIX=1
while [ -e "$FILE" ]; do
    FILE="$OUTPUT_DIR/$BASENAME-$SUFFIX.html"
    SUFFIX=$((SUFFIX + 1))
done

printf '%s' "$CONTENT" > "$FILE"
echo "Saved: $FILE"
