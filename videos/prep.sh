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
    d="$V/.work/proyectos/mi-panel" r=kriollo/versadeploy-demo-ci
    # the self-hosted runner (ci-runner/): a fresh container registered with a token that
    # expires in an hour (a stopped one can't start again: it would register twice)
    if [ -z "$(docker ps -q -f name='^versa-ci-runner$')" ]; then
        docker rm -f versa-ci-runner >/dev/null 2>&1 || true
        docker build -q -t versa-ci-runner "$V/ci-runner" >/dev/null
        token=$(gh api -X POST "repos/$r/actions/runners/registration-token" -q .token)
        docker run -d --name versa-ci-runner --network host -e RUNNER_TOKEN="$token" versa-ci-runner bash -c \
            "./config.sh --url https://github.com/$r --token \"\$RUNNER_TOKEN\" --unattended --name versa-video-pc --replace --disableupdate && ./run.sh" >/dev/null
    fi
    i=0
    until gh api "repos/$r/actions/runners" -q '.runners[].status' | grep -qx online; do
        i=$((i + 1)); [ $i -lt 40 ] || { echo "07: the runner never came online"; exit 1; }; sleep 3
    done
    # the repo before CI (main without a workflow), and no key or secrets for CI yet
    git -C "$d" reset -q --hard ep7-start
    rm -rf "$d/.github" "$d/app/funciones.php"   # what the last recording left uncommitted
    git -C "$d" push -q -f origin main
    for s in DEPLOY_SSH_KEY DEPLOY_KNOWN_HOSTS; do gh secret delete "$s" -R "$r" 2>/dev/null || true; done
    docker exec versa-video sed -i '/ github-actions$/d' /home/deploy/.ssh/authorized_keys
    # the server at that commit, deployed from here as in episodes 5 and 6
    (cd "$d" && HOME="$V/.work/home" PATH="$V/.work/home/.local/bin:$PATH" versa deploy production --no-gui >/dev/null 2>&1)
    # the workflow the video opens (written before recording, like deploy.yml)
    install -D -m 644 "$V/ci-runner/deploy.yml" "$d/.github/workflows/deploy.yml"
    ;;
*) echo "usage: $0 00|02|03|04|05|06|07"; exit 2 ;;
esac
