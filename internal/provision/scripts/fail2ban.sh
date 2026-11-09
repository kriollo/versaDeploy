# Fail2ban: bans what is unambiguously abuse, never an app's normal traffic.
#   sshd        failed SSH logins (systemd journal), bans growing for repeat offenders
#   versa-scan  requests for paths no deployed app serves (.env, .git/, phpMyAdmin,
#               phpunit…) that got 404/403/444: a real app path answers 200 and never counts
# Never banned on: 429 (nginx rate limit), 401/403/422 from apps and APIs, webhooks,
# logins: dashboards, payment callbacks and users behind one NAT IP stay untouched.
# The server-wide jails live in /etc/fail2ban/jail.d/versa.conf (one per server).
conf=/etc/fail2ban/jail.d/versa.conf
if package_installed fail2ban || package_installed fail2ban-server; then
    if [ ! -f "$conf" ]; then
        # set up by someone else: their jails stay as they are
        ok "Fail2ban already installed and configured outside versa — untouched"
        service_active fail2ban || warn "fail2ban is not running — journalctl -u fail2ban -n 30"
        return 0
    fi
    f2b_fresh=""
else
    # the package byte-compiles its own tests: hide their harmless SyntaxWarnings
    export PYTHONWARNINGS=ignore
    if [ "$PKG_MGR" = apt ]; then
        pkg_install fail2ban python3-systemd
    else
        # fail2ban-server without the sendmail action; systemd backend; EPEL
        pkg_install epel-release
        pkg_install fail2ban-server python3-systemd
    fi
    f2b_fresh=1
fi

# Ban through the firewall the server already uses
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then
    banaction=ufw; banall=ufw
elif service_active firewalld; then
    [ "$PKG_MGR" = apt ] || pkg_install fail2ban-firewalld
    banaction=firewallcmd-rich-rules; banall=firewallcmd-allports
else
    command -v nft >/dev/null 2>&1 || pkg_install nftables
    banaction=nftables-multiport; banall=nftables-allports
fi

# Every vhost logs here (nginx's default access_log; Apache: <domain>-access.log)
# Apache's stays a glob, so the next site's log is watched too (install_vhost reloads)
logs=""
for g in /var/log/nginx/access.log '/var/log/apache2/*access.log'; do
    # shellcheck disable=SC2086 # the glob must expand here
    set -- $g
    [ -f "$1" ] && logs="$logs $g"
done

install -d -m 755 /etc/fail2ban/filter.d /etc/fail2ban/jail.d
cat > /etc/fail2ban/filter.d/versa-scan.conf <<'F2B_FILTER'
# Generado por versaDeploy — escáneres de vulnerabilidades (combined log de nginx/Apache).
# Solo cuenta respuestas 404/403/444/400: si la ruta existe en una app (200/30x), no cuenta.
[Definition]
failregex = ^<HOST> \S+ \S+ \[[^\]]*\] "[A-Z]+ /(?:[^" ]*/)?(?i:\.env|\.git/|\.svn/|\.hg/|\.aws/|\.ssh/|\.DS_Store|\.htpasswd|wp-login\.php|xmlrpc\.php|wp-admin/|wp-config|phpmyadmin|pma/|myadmin|adminer|vendor/phpunit/|phpunit/|cgi-bin/|boaform/|HNAP1|actuator/|config\.json\.bak|[^/" ]+\.sql(?:\.gz|\.zip)?\b)[^" ]* HTTP/[\d.]+" (?:400|403|404|444)
ignoreregex =
F2B_FILTER

ignore="127.0.0.1/8 ::1 ${ADMIN_IP:-} ${FAIL2BAN_IGNORE:-}"
tmp="$(mktemp)"
cat > "$tmp" <<F2B_JAIL
# Generado por versaDeploy — no se edita a mano: provision.fail2ban / fail2ban_ignore
[DEFAULT]
# localhost, the admin who provisioned the server, provision.fail2ban_ignore
ignoreip = $(echo $ignore)
bantime = 1h
bantime.increment = true
bantime.maxtime = 1w
findtime = 10m
banaction = ${banaction}
banaction_allports = ${banall}

[sshd]
enabled = true
port = ${SSH_PORT}
backend = systemd
maxretry = 5
F2B_JAIL
if [ -n "$logs" ]; then
    set -f # the Apache glob goes into the config as is
    cat >> "$tmp" <<F2B_SCAN

[versa-scan]
enabled = true
port = http,https
filter = versa-scan
logpath = $(echo $logs | sed 's/ /\n          /g')
backend = auto
maxretry = 3
bantime = 1d
F2B_SCAN
    set +f
else
    warn "No web server access log yet: only the sshd jail (re-run after installing nginx/Apache)"
fi

if cmp -s "$tmp" "$conf"; then
    rm -f "$tmp"
    changed=""
else
    install -m 644 "$tmp" "$conf"
    rm -f "$tmp"
    changed=1
fi
if ! fail2ban-client -t >/dev/null 2>&1; then
    fail2ban-client -t 2>&1 | tail -5
    die "fail2ban rejected the config ($conf)"
fi
systemctl enable fail2ban >/dev/null 2>&1
if [ -n "$f2b_fresh" ] || ! service_active fail2ban; then
    systemctl restart fail2ban
elif [ -n "$changed" ]; then
    fail2ban-client reload >/dev/null   # keeps the current bans
fi
for _ in 1 2 3 4 5 6 7 8 9 10; do fail2ban-client ping >/dev/null 2>&1 && break; sleep 1; done
service_active fail2ban || { journalctl -u fail2ban -n 20 --no-pager || true; die "fail2ban did not start"; }
ok "Fail2ban: $(fail2ban-client status | sed -n 's/.*Jail list:[[:space:]]*//p') (ban via ${banaction}; ignored: $(echo $ignore))"
