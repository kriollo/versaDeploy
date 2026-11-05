# Never install a second PHP-FPM next to an existing one
for other in $(dpkg-query -W -f='${Package}\n' 'php*-fpm' 2>/dev/null | sed -n 's/^php\([0-9]\.[0-9]\)-fpm$/\1/p'); do
    if [ "$other" != "$PHP_VERSION" ] && package_installed "php${other}-fpm"; then
        die "PHP-FPM ${other} is already installed — PHP ${PHP_VERSION} not installed. Set php_version: \"${other}\""
    fi
done
php_prefix="php${PHP_VERSION}"
php_fresh=1
package_installed "${php_prefix}-fpm" && php_fresh=""
if [ -n "$php_fresh" ] && ! apt-cache show "${php_prefix}-fpm" >/dev/null 2>&1; then
    if [ "$OS_ID" = ubuntu ]; then
        info "Adding PPA ondrej/php..."
        add-apt-repository -y ppa:ondrej/php >/dev/null
    else
        info "Adding packages.sury.org/php repository..."
        apt_install ca-certificates curl gnupg
        curl -fsSL https://packages.sury.org/php/apt.gpg -o /usr/share/keyrings/sury-php.gpg
        echo "deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ ${OS_CODENAME} main" > /etc/apt/sources.list.d/sury-php.list
    fi
    apt-get update -qq
    apt-cache show "${php_prefix}-fpm" >/dev/null 2>&1 || die "PHP ${PHP_VERSION} is not available for this OS"
fi

ext=(cli fpm mbstring xml bcmath curl zip gd intl opcache redis)
case "$DATABASE" in
    postgres) ext+=(pgsql) ;;
    mariadb)  ext+=(mysql) ;;
esac
pkgs=()
for e in "${ext[@]}"; do pkgs+=("${php_prefix}-${e}"); done
apt_install "${pkgs[@]}"
if [ -z "$php_fresh" ]; then
    # existing PHP: only the missing extensions; php.ini and OPCache stay as they are
    if [ -n "$APT_INSTALLED" ]; then
        systemctl reload "${php_prefix}-fpm"
        ok "PHP ${PHP_VERSION} already installed — added:${APT_INSTALLED} (php-fpm reloaded)"
    else
        ok "PHP ${PHP_VERSION} already installed with every extension — php.ini/OPCache untouched"
    fi
    return 0
fi
ok "PHP ${PHP_VERSION} installed: ${ext[*]}"

for ini in "/etc/php/${PHP_VERSION}/fpm/php.ini" "/etc/php/${PHP_VERSION}/cli/php.ini"; do
    [ -f "$ini" ] || continue
    sed -i "s|^;*date\.timezone\s*=.*|date.timezone = ${PHP_TIMEZONE:-America/Santiago}|" "$ini"
    sed -i "s|^upload_max_filesize\s*=.*|upload_max_filesize = 64M|" "$ini"
    sed -i "s|^post_max_size\s*=.*|post_max_size = 64M|" "$ini"
    sed -i "s|^memory_limit\s*=.*|memory_limit = 256M|" "$ini"
    sed -i "s|^max_execution_time\s*=.*|max_execution_time = 300|" "$ini"
    sed -i "s|^expose_php\s*=.*|expose_php = Off|" "$ini"
    ok "$(basename "$(dirname "$ini")")/php.ini tuned"
done

# OPCache for zero-downtime symlink deploys: revalidate_path=1 keys the cache by the real
# path, so /current → releases/vN never serves the previous release's bytecode.
cat > "/etc/php/${PHP_VERSION}/fpm/conf.d/99-opcache-production.ini" <<'OPCACHE_EOF'
; OPCache — producción (generado por versaDeploy)
[opcache]
opcache.enable=1
opcache.enable_cli=0
opcache.memory_consumption=128
opcache.interned_strings_buffer=16
opcache.max_accelerated_files=10000
opcache.validate_timestamps=0
opcache.revalidate_freq=0
opcache.revalidate_path=1
opcache.save_comments=1
opcache.huge_code_pages=0
opcache.jit_buffer_size=64M
opcache.jit=tracing
OPCACHE_EOF
ok "OPCache configured"

systemctl enable "${php_prefix}-fpm" >/dev/null 2>&1
systemctl restart "${php_prefix}-fpm"
service_active "${php_prefix}-fpm" || die "${php_prefix}-fpm failed to start — journalctl -u ${php_prefix}-fpm -n 30"
ok "${php_prefix}-fpm running"
