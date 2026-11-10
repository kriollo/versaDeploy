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
# GitHub Actions: the demo repo back at its base commit, deployed by its own pipeline,
# and the self-hosted runner up (ci-runner/; see README)
07) curl -fsS -o /dev/null http://mipanel.localhost:8099/up || { echo "07 continues 05/06: ./make.sh 05 first"; exit 1; }
    docker start versa-ci-runner >/dev/null
    d="$V/.work/proyectos/mi-panel"
    git -C "$d" reset -q --hard ep7-base
    if [ "$(git -C "$d" rev-parse HEAD)" != "$(git -C "$d" ls-remote origin main | cut -f1)" ]; then
        git -C "$d" push -q -f origin main
        sleep 15
        id=$(gh run list -R kriollo/versadeploy-demo-ci -w Deploy -L 1 --json databaseId -q '.[0].databaseId')
        gh run watch "$id" -R kriollo/versadeploy-demo-ci --exit-status >/dev/null
    fi
    ;;
*) echo "usage: $0 00|02|03|04|05|06|07"; exit 2 ;;
esac
