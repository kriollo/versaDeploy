# Dragonfly: Redis-compatible (same protocol and port), multi-threaded. Never next to
# Redis: both want 127.0.0.1:6379.
if package_installed redis-server || package_installed redis; then
    die "Redis is already installed — Dragonfly not installed (both use port 6379). Keep Redis (redis: true)"
fi
if package_installed dragonfly; then
    # an existing Dragonfly keeps its config and keeps running
    ok "Dragonfly already installed ($(dragonfly --version 2>/dev/null | head -1 | awk '{print $2}' | tr -d '\033' | sed 's/\[[0-9;]*m//g; s/-.*//')) — config untouched, no restart"
    service_active dragonfly || warn "dragonfly is not running — journalctl -u dragonfly -n 30"
    return 0
fi

# Official release packages, pinned: bump the version and the sha256 together
# (gh api repos/dragonflydb/dragonfly/releases/tags/vX --jq '.assets[]|.name+" "+.digest')
ver=v2.1
case "${PKG_MGR}/$(uname -m)" in
    apt/x86_64)  f=dragonfly_amd64.deb;  sum=55e199473016a30285aa43f9879847693a527f7213d8f894a0778bd79a0fb5d3 ;;
    apt/aarch64) f=dragonfly_arm64.deb;  sum=9af027a23e83fdfa8ba9f9cefde9db3b3c84714f05dc3a16b6571189147d9e42 ;;
    dnf/x86_64)  f=dragonfly.x86_64.rpm; sum=e45eea9c287f76e487cbce86646fe8e4b8340035dfac1ad15a228410314c4ea1 ;;
    *) die "Dragonfly ${ver} has no package for ${PKG_MGR} on $(uname -m): use Redis (redis: true)" ;;
esac
tmp="$(mktemp -d)"
curl -fsSL --proto '=https' -o "$tmp/$f" "https://github.com/dragonflydb/dragonfly/releases/download/${ver}/${f}" \
    || { rm -rf "$tmp"; die "could not download Dragonfly ${ver}"; }
echo "${sum}  ${tmp}/${f}" | sha256sum -c --quiet >/dev/null 2>&1 || { rm -rf "$tmp"; die "Dragonfly ${f}: sha256 mismatch — not installed"; }
# The package's flagfile listens on every interface and phones home for new versions.
# Ours: localhost, 256 MB, evicts like allkeys-lru (same footprint as the Redis step).
# Dragonfly needs 256 MB per thread: more memory → raise proactor_threads with it.
write_dfly_conf() {
    mkdir -p /etc/dragonfly
    cat > /etc/dragonfly/dragonfly.conf <<'DFLY_EOF'
# Generado por versaDeploy — solo conexiones locales
--bind=127.0.0.1
--port=6379
--maxmemory=256mb
--proactor_threads=1
--cache_mode=true
--dir=/var/lib/dragonfly
--dbfilename=dump
--log_dir=/var/log/dragonfly
--max_log_size=20
--pidfile=/var/run/dragonfly/dragonfly.pid
--version_check=false
DFLY_EOF
}
if [ "$PKG_MGR" = apt ]; then
    # the .deb starts the service on install: ours must be in place first (kept as conffile)
    write_dfly_conf
    apt-get install -y -qq -o Dpkg::Options::=--force-confold "$tmp/$f"
else
    # the .rpm doesn't start it, and overwrites the file (not %config): write it after
    dnf install -y -q "$tmp/$f"
    write_dfly_conf
fi
rm -rf "$tmp"
APT_INSTALLED="$APT_INSTALLED dragonfly"

install -d -o dfly -g dfly -m 750 /var/lib/dragonfly /var/log/dragonfly
systemctl daemon-reload
systemctl enable dragonfly >/dev/null 2>&1
if systemctl restart dragonfly && sleep 1 && service_active dragonfly; then
    ok "Dragonfly ${ver} installed (127.0.0.1:6379, maxmemory 256mb, cache mode)"
else
    journalctl -u dragonfly -n 20 --no-pager || true
    die "dragonfly did not start — journalctl -xeu dragonfly"
fi
