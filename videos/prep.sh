#!/bin/sh
# prep.sh <episode> — the state an episode starts from: a fresh "VPS" and the project.
set -eu
V=$(cd "$(dirname "$0")" && pwd)
case $1 in
00) rm -f "$V/.work/home/.local/bin/versa" ;;
02) "$V/server.sh" up && "$V/demos.sh" laravel ;;
03) "$V/server.sh" up && "$V/demos.sh" fastapi ;;
04) "$V/server.sh" up && "$V/demos.sh" nuxt ;;
# the tape creates the project (composer create-project)
05) "$V/server.sh" up && rm -rf "$V/.work/proyectos/mi-panel" ;;
# starts where 05 ends: the panel deployed and installed
06) curl -fsS -o /dev/null http://mipanel.localhost:8099/up || { echo "06 continues 05: ./make.sh 05 first"; exit 1; }
    if git -C "$V/.work/proyectos/mi-panel" status --porcelain | grep -q .; then echo "mi-panel has uncommitted changes"; exit 1; fi
    ;;
*) echo "usage: $0 00|02|03|04|05|06"; exit 2 ;;
esac
