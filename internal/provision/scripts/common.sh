#!/usr/bin/env bash
# Generado por versaDeploy — port no interactivo de setup-vps.sh. Corre como root.
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck source=/dev/null
. ./env
export DEBIAN_FRONTEND=noninteractive
export PATH="$PATH:/usr/sbin:/sbin"

info()    { echo "[INFO]  $*"; }
ok()      { echo "[OK]    $*"; }
warn()    { echo "[WARN]  $*"; }
die()     { echo "[ERROR] $*"; exit 1; }
section() { echo; echo "══════ $* ══════"; }

package_installed() {
    if [ "$PKG_MGR" = apt ]; then dpkg -l "$1" 2>/dev/null | grep -q '^ii'; else rpm -q "$1" >/dev/null 2>&1; fi
}
# pkg_available NAME: installed, or installable from the enabled repos
pkg_available() {
    package_installed "$1" && return 0
    if [ "$PKG_MGR" = apt ]; then apt-cache show "$1" >/dev/null 2>&1; else dnf list -q available "$1" >/dev/null 2>&1; fi
}
# dnf_candidate NAME: version dnf would install (after module streams), e.g. 8.3.19
dnf_candidate() { dnf repoquery -q --latest-limit 1 --qf '%{version}' "$1" 2>/dev/null | head -1; }
service_active()    { systemctl is-active --quiet "$1" 2>/dev/null; }
# apt_install installs only the packages that are missing: `apt-get install` of an
# installed package would upgrade it, and upgrades only happen on request (U/S).
# APT_INSTALLED lists what this run actually installed.
APT_INSTALLED=""
pkg_install() {
    local p missing=()
    for p in "$@"; do package_installed "$p" || missing+=("$p"); done
    [ ${#missing[@]} -eq 0 ] && return 0
    if [ "$PKG_MGR" = apt ]; then
        apt-get install -y -qq -o Dpkg::Options::=--force-confold "${missing[@]}"
    else
        dnf install -y -q "${missing[@]}"
    fi
    APT_INSTALLED="$APT_INSTALLED ${missing[*]}"
}
apt_install() { pkg_install "$@"; }

# fpm_umask UNIT: PHP-FPM creates group-writable files (cache, logs, uploads), so the
# deploy user's CLI and cron can clean and reuse them, and the other way round
fpm_umask() {
    mkdir -p "/etc/systemd/system/$1.service.d"
    printf '[Service]\nUMask=0002\n' > "/etc/systemd/system/$1.service.d/versa-umask.conf"
    systemctl daemon-reload
}

# SELinux (RHEL family): a persistent file context, applied now. No-op without SELinux.
selinux_on() { command -v selinuxenabled >/dev/null 2>&1 && selinuxenabled; }
selinux_label() { # TYPE PATTERN PATH
    selinux_on || return 0
    command -v semanage >/dev/null 2>&1 || pkg_install policycoreutils-python-utils
    semanage fcontext -a -t "$1" "$2" 2>/dev/null || semanage fcontext -m -t "$1" "$2"
    restorecon -R "$3"
}

[ "$(id -u)" = 0 ] || die "must run as root (set provision.ssh.user: root in deploy.yml)"
if command -v apt-get >/dev/null 2>&1; then
    PKG_MGR=apt
elif command -v dnf >/dev/null 2>&1; then
    PKG_MGR=dnf
else
    die "installation supports apt (Debian/Ubuntu) or dnf (RHEL/Rocky/Alma 8+) servers"
fi
OS_ID="$(. /etc/os-release 2>/dev/null; echo "${ID:-}")"
OS_CODENAME="$(. /etc/os-release 2>/dev/null; echo "${VERSION_CODENAME:-}")"

APT_UPDATED=""
pkg_update_once() {
    [ -n "$APT_UPDATED" ] && return 0
    if [ "$PKG_MGR" = apt ]; then apt-get update -qq; else dnf makecache -q >/dev/null 2>&1 || true; fi
    APT_UPDATED=1
}
apt_update_once() { pkg_update_once; }
pkg_update_once

# install_vhost FILE: installs ./FILE as the project's vhost, validates and reloads.
# Keeps a timestamped backup and restores it when the new config fails validation.
install_vhost() {
    local src="$1" dst bak=""
    if [ "$WEB_SERVER" = nginx ]; then
        if [ -d /etc/nginx/sites-available ]; then
            dst="/etc/nginx/sites-available/${PROJECT_NAME}"
        else
            dst="/etc/nginx/conf.d/${PROJECT_NAME}.conf"
        fi
    else
        dst="/etc/apache2/sites-available/${PROJECT_NAME}.conf"
    fi
    # Never create DOC_ROOT: it lives under current/, which must stay a symlink that the
    # first deploy creates (a real directory there makes the symlink switch fail).
    mkdir -p "$REMOTE_PATH"
    if [ -f "$dst" ]; then
        bak="${dst}.bak.$(date +%Y%m%d%H%M%S)"
        cp "$dst" "$bak"
        info "Previous vhost backed up: $bak"
    fi
    # Refuse to add a second vhost for a domain another site already serves (e.g. a vhost
    # made by hand or by certbot under another name): both would fight over the domain.
    local d="${DOMAIN//./\\.}" other
    other="$(grep -lE "^[[:space:]]*(server_name|ServerName|ServerAlias)[[:space:]](.*[[:space:]])?${d}([[:space:];]|$)" \
        /etc/nginx/sites-enabled/* /etc/apache2/sites-enabled/* 2>/dev/null \
        | grep -vE "/${PROJECT_NAME}(\.conf)?$" || true)"
    if [ -n "$other" ]; then
        die "${DOMAIN} is already served by: $(echo $other) — set provision project/domain to match it or disable it first"
    fi
    # 644: the bundle files are 0600, and the deploy user's detection reads vhosts
    install -m 644 "$src" "$dst"
    if [ "$WEB_SERVER" = nginx ]; then
        # 'http2 on;' needs nginx >= 1.25.1; older versions take it on the listen line
        local v; v="$(nginx -v 2>&1 | sed 's|.*/||')"
        if [ "$(printf '%s\n' "$v" 1.25.1 | sort -V | head -1)" != 1.25.1 ]; then
            sed -i 's|^\(\s*\)http2 on;|\1# http2 on; (nginx < 1.25.1)|; s|listen 443 ssl;|listen 443 ssl http2;|' "$dst"
        fi
        if [ -d /etc/nginx/sites-enabled ]; then
            ln -sfn "$dst" "/etc/nginx/sites-enabled/${PROJECT_NAME}"
            [ -n "${WEB_FRESH:-}" ] && rm -f /etc/nginx/sites-enabled/default
        fi
        if ! nginx -t 2>&1; then
            [ -n "$bak" ] && cp "$bak" "$dst" || rm -f "$dst" "/etc/nginx/sites-enabled/${PROJECT_NAME}"
            die "nginx -t failed — previous config restored"
        fi
        systemctl reload nginx 2>/dev/null || systemctl restart nginx
    else
        a2ensite "${PROJECT_NAME}" >/dev/null
        [ -n "${WEB_FRESH:-}" ] && { a2dissite 000-default >/dev/null 2>&1 || true; }
        if ! apache2ctl configtest 2>&1; then
            [ -n "$bak" ] && cp "$bak" "$dst" || { a2dissite "${PROJECT_NAME}" >/dev/null 2>&1; rm -f "$dst"; }
            die "apache2ctl configtest failed — previous config restored"
        fi
        systemctl reload apache2 2>/dev/null || systemctl restart apache2
    fi
    ok "Vhost installed: $dst ($src)"
}

# install_site_config picks the SSL vhost when the certificate already exists, so
# re-running the web server step never downgrades a site to plain HTTP.
install_site_config() {
    if [ -f "$CERT_DIR/fullchain.pem" ]; then
        ensure_ssl_includes
        install_vhost vhost-ssl.conf
    else
        install_vhost vhost.conf
    fi
}

# ensure_ssl_includes creates the files certbot's installer normally writes, which
# `certbot certonly` may skip.
ensure_ssl_includes() {
    mkdir -p /etc/letsencrypt
    if [ ! -f /etc/letsencrypt/ssl-dhparams.pem ]; then
        info "Generating /etc/letsencrypt/ssl-dhparams.pem..."
        openssl dhparam -dsaparam -out /etc/letsencrypt/ssl-dhparams.pem 2048 2>/dev/null
    fi
    if [ ! -f /etc/letsencrypt/options-ssl-nginx.conf ]; then
        cat > /etc/letsencrypt/options-ssl-nginx.conf <<'SSLN'
ssl_session_cache shared:le_nginx_SSL:10m;
ssl_session_timeout 1440m;
ssl_session_tickets off;
ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers off;
ssl_ciphers "ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384";
SSLN
    fi
    if [ ! -f /etc/letsencrypt/options-ssl-apache.conf ]; then
        cat > /etc/letsencrypt/options-ssl-apache.conf <<'SSLA'
SSLEngine on
SSLProtocol all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1
SSLCipherSuite ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384
SSLHonorCipherOrder off
SSLSessionTickets off
SSLOptions +StrictRequire
SSLA
    fi
}
