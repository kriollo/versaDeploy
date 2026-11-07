# Never next to Dragonfly: both want 127.0.0.1:6379
package_installed dragonfly && die "Dragonfly is already installed — Redis not installed (both use port 6379). Keep Dragonfly (dragonfly: true)"
if [ "$PKG_MGR" = apt ]; then redis_pkg=redis-server; redis_svc=redis-server; else redis_pkg=redis; redis_svc=redis; fi
conf=/etc/redis/redis.conf
if package_installed "$redis_pkg"; then
    # an existing Redis keeps its config (maxmemory/eviction may matter to other apps)
    maxmem="$(redis-cli CONFIG GET maxmemory 2>/dev/null | tail -1)"
    ok "Redis already installed — config untouched, no restart (maxmemory=${maxmem:-?})"
    service_active $redis_svc || warn "$redis_svc is not running — journalctl -u $redis_svc -n 30"
    return 0
fi
pkg_install "$redis_pkg"
bind_line="bind 127.0.0.1 -::1"
if [ "$(cat /proc/sys/net/ipv6/conf/all/disable_ipv6 2>/dev/null)" = 1 ]; then
    bind_line="bind 127.0.0.1"
    info "IPv6 disabled — Redis binds 127.0.0.1 only"
fi
if [ -f "$conf" ]; then
    cat >> "$conf" <<REDIS_OVERRIDE

# === versaDeploy override === #
${bind_line}
protected-mode yes
port 6379
maxmemory 256mb
maxmemory-policy allkeys-lru
supervised auto
loglevel notice
# === end versaDeploy override ===
REDIS_OVERRIDE
fi
systemctl daemon-reload
systemctl enable $redis_svc >/dev/null 2>&1
if systemctl restart $redis_svc && service_active $redis_svc; then
    ok "Redis installed (127.0.0.1, maxmemory 256mb, allkeys-lru)"
else
    journalctl -u $redis_svc -n 20 --no-pager || true
    warn "$redis_svc did not start — check: journalctl -xeu $redis_svc"
fi
