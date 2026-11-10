# FrankenPHP: PHP app server (static binary with PHP and its extensions) run by the
# deploy as a service (deploy.yml services); nginx serves the static files and proxies
# index.php to it. Never next to PHP-FPM: one PHP per server.
if ls /usr/sbin/php*-fpm* /usr/sbin/php-fpm >/dev/null 2>&1 || package_installed php-fpm; then
    die "PHP-FPM is already installed — FrankenPHP not installed (one PHP per server). Keep PHP-FPM (remove frankenphp: true)"
fi
[ "$WEB_SERVER" = nginx ] || die "FrankenPHP is served behind nginx (web_server: nginx)"

bin=/usr/local/bin/frankenphp
if [ -x "$bin" ]; then
    # an existing FrankenPHP keeps its version (upgrades: replace the binary by hand)
    ok "FrankenPHP already installed ($("$bin" version 2>/dev/null | awk '{print $2, $3, $4}'))"
else
    # Official static build (PHP + the common extensions), pinned: bump the version and
    # the sha256 together (gh api repos/php/frankenphp/releases/tags/vX --jq '.assets[]|.name+" "+.digest')
    ver=v1.13.4
    case "$(uname -m)" in
        x86_64)  f=frankenphp-linux-x86_64;  sum=88e4229fb48167418e50bc29b94df5a4f83801ec70ea6637ed44bf0527e74c29 ;;
        aarch64) f=frankenphp-linux-aarch64; sum=c405bbb9c6561381109a12c5707ec71109f098423c3c935667543c2c443e37f8 ;;
        *) die "FrankenPHP ${ver} has no build for $(uname -m): use PHP-FPM" ;;
    esac
    tmp="$(mktemp -d)"
    curl -fsSL --proto '=https' -o "$tmp/$f" "https://github.com/php/frankenphp/releases/download/${ver}/${f}" \
        || { rm -rf "$tmp"; die "could not download FrankenPHP ${ver}"; }
    echo "${sum}  ${tmp}/${f}" | sha256sum -c --quiet >/dev/null 2>&1 || { rm -rf "$tmp"; die "FrankenPHP ${f}: sha256 mismatch — not installed"; }
    install -m 755 "$tmp/$f" "$bin"
    rm -rf "$tmp"
    selinux_label bin_t "$bin" "$bin"
    ok "FrankenPHP ${ver} installed ($("$bin" version | awk '{print $3, $4}'))"
fi

# `php` for hooks and cron (php artisan migrate): the binary's CLI, unless a PHP CLI exists.
# exec -a: PHP_BINARY is argv[0], so PHP that runs PHP_BINARY (artisan schedule:run,
# versaCLI app:deploy:finish) runs this wrapper instead of `frankenphp <script>`.
if ! command -v php >/dev/null 2>&1 || grep -q 'Generado por versaDeploy' /usr/local/bin/php 2>/dev/null; then
    printf '#!/bin/bash\n# Generado por versaDeploy: PHP CLI de FrankenPHP\nexec -a /usr/local/bin/php %s php-cli "$@"\n' "$bin" > /usr/local/bin/php
    chmod 755 /usr/local/bin/php
    ok "php → frankenphp php-cli (/usr/local/bin/php; php -v/-m need PHP 8.6)"
fi

# The static build has no php.ini: PHP's defaults print every error and deprecation, on
# web pages too. Production settings for the server and the CLI (hooks, cron); files
# added to php.d after this one override it.
install -d -m 755 /etc/frankenphp/php.d
printf '; Generado por versaDeploy: PHP de producción (servidor y CLI)\ndisplay_errors = Off\ndisplay_startup_errors = Off\nlog_errors = On\nerror_reporting = E_ALL & ~E_DEPRECATED\ndate.timezone = %s\n' \
    "${PHP_TIMEZONE:-America/Santiago}" > /etc/frankenphp/php.d/00-versa.ini
chmod 644 /etc/frankenphp/php.d/00-versa.ini

# opcache_reset() endpoint, reached only through each project's owner-only socket:
# deploys free the previous release's scripts with it, without restarting PHP
install -d -m 755 /etc/frankenphp/opcache-reset
printf '<?php\n// Generado por versaDeploy: libera OPcache tras un deploy (socket del proyecto, solo su dueño)\necho opcache_reset() ? "opcache reset\\n" : "opcache disabled\\n";\n' > /etc/frankenphp/opcache-reset/index.php
chmod 644 /etc/frankenphp/opcache-reset/index.php

# The project's server config (the service runs it): 127.0.0.1 only, every request runs
# index.php, like the FPM vhost. Rewritten on each run, like the vhost.
install -d -m 755 /etc/frankenphp
install -m 644 Caddyfile "/etc/frankenphp/${PROJECT_NAME}.Caddyfile"
"$bin" validate --config "/etc/frankenphp/${PROJECT_NAME}.Caddyfile" >/dev/null 2>&1 \
    || die "invalid /etc/frankenphp/${PROJECT_NAME}.Caddyfile: $("$bin" validate --config "/etc/frankenphp/${PROJECT_NAME}.Caddyfile" 2>&1 | tail -1)"
ok "Config: /etc/frankenphp/${PROJECT_NAME}.Caddyfile (127.0.0.1:${PROXY_PORT}) — the deploy starts the service"
