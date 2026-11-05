conf=/etc/redis/redis.conf
if package_installed redis-server; then
    # an existing Redis keeps its config (maxmemory/eviction may matter to other apps)
    maxmem="$(redis-cli CONFIG GET maxmemory 2>/dev/null | tail -1)"
    ok "Redis already installed — config untouched, no restart (maxmemory=${maxmem:-?})"
    service_active redis-server || warn "redis-server is not running — journalctl -u redis-server -n 30"
    return 0
fi
apt_install redis-server
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
systemctl enable redis-server >/dev/null 2>&1
if systemctl restart redis-server && service_active redis-server; then
    ok "Redis installed (127.0.0.1, maxmemory 256mb, allkeys-lru)"
else
    journalctl -u redis-server -n 20 --no-pager || true
    warn "redis-server did not start — check: journalctl -xeu redis-server"
fi
