#!/bin/sh
# shot.sh URL OUT.png — the page as a browser shows it (headless Chrome with a
# throwaway profile: never your own), 1600x900.
set -eu
chrome=$(command -v google-chrome || command -v chromium || command -v chromium-browser)
profile=$(mktemp -d)
trap 'rm -rf "$profile"' EXIT
"$chrome" --headless=new --disable-gpu --hide-scrollbars --no-first-run --user-data-dir="$profile" \
    --window-size=1600,900 --virtual-time-budget=4000 --screenshot="$2" "$1" >/dev/null 2>&1
[ -s "$2" ] || { echo "shot.sh: no screenshot of $1"; exit 1; }
