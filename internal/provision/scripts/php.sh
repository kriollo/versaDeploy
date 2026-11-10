# PHP-FPM of this project's version. Debian/Ubuntu install each version next to the
# others (php8.1-fpm, php8.4-fpm, FrankenPHP too): the deploy points the project's
# <remote_path>/.versa/bin/php at its version, so other projects keep theirs.
# RHEL family: PHP from the AppStream module stream php:X.Y, one php-fpm unit
if [ "$PKG_MGR" != apt ]; then
    if package_installed php-fpm; then
        cur="$(php-fpm -v 2>/dev/null | sed -n '1s/^PHP \([0-9]*\.[0-9]*\).*/\1/p')"
        [ "$cur" = "$PHP_VERSION" ] || die "PHP-FPM ${cur:-?} is already installed — PHP ${PHP_VERSION} not installed. Set php_version: \"${cur}\""
        php_fresh=""
    else
        php_fresh=1
        # switching the stream is only safe while no PHP is installed
        dnf module reset -y -q php >/dev/null 2>&1 || true
        dnf module enable -y -q "php:${PHP_VERSION}" >/dev/null 2>&1 || true
        cand="$(dnf_candidate php-fpm | cut -d. -f1,2)"
        el="$(. /etc/os-release; echo "${VERSION_ID%%.*}")"
        if { [ "$cand" != "$PHP_VERSION" ] || [ "${INSTALL_REDIS:-false}" = true ]; } && [ "$OS_ID" != fedora ] && [ ! -f /etc/yum.repos.d/remi-modular.repo ]; then
            # like ondrej/sury on Debian: Remi's repo has every supported PHP for EL 8+, and
            # phpredis for each (Redis/Dragonfly need it)
            info "Adding Remi's repository (rpms.remirepo.net) for PHP ${PHP_VERSION}..."
            pkg_install epel-release
            dnf install -y -q "https://rpms.remirepo.net/enterprise/remi-release-${el}.rpm"
            dnf module reset -y -q php >/dev/null 2>&1 || true
            dnf module enable -y -q "php:remi-${PHP_VERSION}" >/dev/null 2>&1 || true
            cand="$(dnf_candidate php-fpm | cut -d. -f1,2)"
        fi
        [ "$cand" = "$PHP_VERSION" ] || die "PHP ${PHP_VERSION} is not available for ${OS_ID} (its repos offer ${cand:-none}) — set php_version: \"${cand}\""
    fi
    pkgs=()
    # posix/pcntl (php-process) and sodium ship apart from php-common on RHEL, unlike Debian
    for p in php-fpm php-cli php-mbstring php-xml php-bcmath php-gd php-intl php-opcache php-pecl-zip php-process php-sodium; do
        pkg_available "$p" && pkgs+=("$p")
    done
    # phpredis: Remi builds it for every stream; EPEL's only fits the default (non-module) PHP
    if [ -f /etc/yum.repos.d/remi-modular.repo ]; then
        pkgs+=(php-pecl-redis6)
    elif [ "${INSTALL_REDIS:-false}" = true ]; then
        warn "phpredis is not packaged for PHP ${PHP_VERSION} on ${OS_ID}: Redis/Dragonfly need Remi's repo (rpms.remirepo.net)"
    fi
    case "$DATABASE" in
        postgres) pkgs+=(php-pgsql) ;;
        mariadb)  pkgs+=(php-mysqlnd) ;;
    esac
    pkg_install "${pkgs[@]}"
    if [ -z "$php_fresh" ]; then
        # existing PHP: only the missing extensions; php.ini, the pool and OPCache stay as they are
        if [ -n "$APT_INSTALLED" ]; then
            systemctl reload php-fpm
            ok "PHP ${PHP_VERSION} already installed — added:${APT_INSTALLED} (php-fpm reloaded)"
        else
            ok "PHP ${PHP_VERSION} already installed with every extension — php.ini/OPCache untouched"
        fi
        return 0
    fi
    ok "PHP ${PHP_VERSION} installed:${APT_INSTALLED}"

    ini=/etc/php.ini
    sed -i "s|^;*date\.timezone\s*=.*|date.timezone = ${PHP_TIMEZONE:-America/Santiago}|" "$ini"
    sed -i "s|^upload_max_filesize\s*=.*|upload_max_filesize = 64M|" "$ini"
    sed -i "s|^post_max_size\s*=.*|post_max_size = 64M|" "$ini"
    sed -i "s|^memory_limit\s*=.*|memory_limit = 256M|" "$ini"
    sed -i "s|^max_execution_time\s*=.*|max_execution_time = 300|" "$ini"
    sed -i "s|^expose_php\s*=.*|expose_php = Off|" "$ini"
    ok "php.ini tuned"
    cat > /etc/php.d/99-opcache-production.ini <<'OPCACHE_EOF'
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

    # Same socket and unit name as Debian (the vhost templates and services_reload use
    # them). /run is a tmpfs: tmpfiles.d recreates /run/php on every boot.
    echo 'd /run/php 0755 root root -' > /etc/tmpfiles.d/versa-php.conf
    systemd-tmpfiles --create /etc/tmpfiles.d/versa-php.conf
    selinux_label httpd_var_run_t '/run/php(/.*)?' /run/php
    # The pool keeps running as apache:apache (sessions live in /var/lib/php/session,
    # group apache) and joins www-data, the group that owns the project dirs (group-
    # writable for logs, cache and uploads). nginx reaches the socket through the stock
    # listen.acl_users.
    getent group www-data >/dev/null || groupadd -r www-data
    usermod -aG www-data apache
    sed -i "s|^listen\s*=.*|listen = /run/php/php${PHP_VERSION}-fpm.sock|" /etc/php-fpm.d/www.conf
    ln -sfn /usr/lib/systemd/system/php-fpm.service "/etc/systemd/system/php${PHP_VERSION}-fpm.service"
    fpm_umask php-fpm
    systemctl enable php-fpm >/dev/null 2>&1
    systemctl restart php-fpm
    service_active php-fpm || die "php-fpm failed to start — journalctl -u php-fpm -n 30"
    ok "php-fpm running (php${PHP_VERSION}-fpm, /run/php/php${PHP_VERSION}-fpm.sock)"
    return 0
fi
others="$(dpkg-query -W -f='${db:Status-Abbrev} ${Package}\n' 'php*-fpm' 2>/dev/null | sed -n 's/^ii  *php\([0-9]\.[0-9]\)-fpm$/\1/p' | grep -vx "$PHP_VERSION" | tr '\n' ' ' || true)"
[ -n "$others" ] && info "PHP-FPM ${others}already installed for other projects: PHP ${PHP_VERSION} goes next to it"
# The server's `php` stays what it was: Debian's alternatives would switch it to the
# newest version installed, under the other projects' hooks and cron jobs
prev_alt=""
for a in php phar phar.phar; do
    t="$(readlink -f "/etc/alternatives/$a" 2>/dev/null || true)"
    [ -n "$t" ] && prev_alt="$prev_alt $a=$t"
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
for e in "${ext[@]}"; do
    # PHP 8.5+ builds OPcache in: there's no php8.5-opcache package
    [ "$e" = opcache ] && ! pkg_available "${php_prefix}-opcache" && continue
    pkgs+=("${php_prefix}-${e}")
done
apt_install "${pkgs[@]}"
for kv in $prev_alt; do
    a="${kv%%=*}"; t="${kv#*=}"
    if [ -x "$t" ] && [ "$(readlink -f "/etc/alternatives/$a")" != "$t" ]; then
        update-alternatives --quiet --set "$a" "$t" && ok "$a (the server's default) is still $(basename "$t")"
    fi
done
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

fpm_umask "${php_prefix}-fpm"
systemctl enable "${php_prefix}-fpm" >/dev/null 2>&1
systemctl restart "${php_prefix}-fpm"
service_active "${php_prefix}-fpm" || die "${php_prefix}-fpm failed to start — journalctl -u ${php_prefix}-fpm -n 30"
ok "${php_prefix}-fpm running"
