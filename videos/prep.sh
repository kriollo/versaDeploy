#!/bin/sh
# prep.sh <episode> — the state an episode starts from: a fresh "VPS" and the project.
set -eu
V=$(cd "$(dirname "$0")" && pwd)
case $1 in
00) rm -f "$V/.work/home/.local/bin/versa" ;;
02) "$V/server.sh" up && "$V/demos.sh" laravel ;;
03) "$V/server.sh" up && "$V/demos.sh" fastapi ;;
04) "$V/server.sh" up && "$V/demos.sh" nuxt ;;
*) echo "usage: $0 00|02|03|04"; exit 2 ;;
esac
