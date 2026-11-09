#!/bin/sh
# server.sh up|down — the "VPS" the episodes deploy to: a fresh Ubuntu 24.04 container
# with systemd and sshd (the e2e image), reachable as root with the sandbox's key.
#   SSH  127.0.0.1:2299   HTTP 127.0.0.1:8099 (http://<app>.localhost:8099)
# The key and known_hosts go into the sandbox HOME (.work/home/.ssh), like on a laptop.
set -eu
V=$(cd "$(dirname "$0")" && pwd)
name=versa-video
home=$V/.work/home
case ${1:-} in
up)
    mkdir -p "$home/.ssh" "$V/.work/ctx"
    chmod 700 "$home/.ssh"
    [ -f "$home/.ssh/id_ed25519" ] || ssh-keygen -q -t ed25519 -N "" -C "dev@laptop" -f "$home/.ssh/id_ed25519"
    cp "$home/.ssh/id_ed25519.pub" "$V/.work/ctx/id.pub"
    docker build -q -t versa-video-server -f "$V/../test/e2e/Dockerfile" "$V/.work/ctx" >/dev/null
    docker rm -f "$name" >/dev/null 2>&1 || true
    docker run -d --name "$name" --hostname vps --privileged --cgroupns=host \
        -v /sys/fs/cgroup:/sys/fs/cgroup:rw --tmpfs /run --tmpfs /run/lock \
        -p 127.0.0.1:2299:22 -p 127.0.0.1:8099:80 versa-video-server >/dev/null
    for _ in $(seq 1 45); do
        keys=$(ssh-keyscan -p 2299 127.0.0.1 2>/dev/null) && [ -n "$keys" ] && break
        sleep 2
    done
    [ -n "${keys:-}" ] || { echo "sshd not reachable"; exit 1; }
    # a real VPS keeps its host key: the old one goes, as after reinstalling it
    ssh-keygen -R "[127.0.0.1]:2299" -f "$home/.ssh/known_hosts" >/dev/null 2>&1 || true
    echo "$keys" >> "$home/.ssh/known_hosts"
    echo "server up: ssh -p 2299 root@127.0.0.1 (key $home/.ssh/id_ed25519)"
    ;;
down)
    docker rm -f "$name" >/dev/null 2>&1 || true
    ;;
*)
    echo "usage: $0 up|down"; exit 2 ;;
esac
