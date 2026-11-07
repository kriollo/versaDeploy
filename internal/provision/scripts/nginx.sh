# Never install an alternative web server: both want ports 80/443
if package_installed apache2; then
    die "Apache is already installed — nginx not installed. Use the Apache step (web_server: apache)"
fi
package_installed nginx || package_installed nginx-core || WEB_FRESH=1
if [ "$PKG_MGR" = apt ]; then
    apt_install nginx libnginx-mod-http-headers-more-filter
else
    # RHEL family: headers-more lives in EPEL
    pkg_install epel-release
    pkg_install nginx nginx-mod-headers-more
    # The vhost template includes Debian's snippet: the standard CGI params (it sets
    # SCRIPT_FILENAME and DOCUMENT_ROOT itself, with $realpath_root)
    if [ ! -f /etc/nginx/snippets/fastcgi-php.conf ]; then
        mkdir -p /etc/nginx/snippets
        echo 'include fastcgi_params;' > /etc/nginx/snippets/fastcgi-php.conf
        ok "Created /etc/nginx/snippets/fastcgi-php.conf"
    fi
    if [ -n "${WEB_FRESH:-}" ] && systemctl is-active --quiet firewalld 2>/dev/null; then
        firewall-cmd -q --permanent --add-service=http --add-service=https && firewall-cmd -q --reload
        ok "firewalld: http and https allowed"
    fi
    # SELinux: nginx may connect to the app's port (proxy_pass)
    if [ "${PROXY_PORT:-0}" != 0 ] && selinux_on; then
        setsebool -P httpd_can_network_connect 1
        ok "SELinux: httpd_can_network_connect on (proxy to 127.0.0.1:${PROXY_PORT})"
    fi
fi
if [ -n "${WEB_FRESH:-}" ]; then ok "Nginx installed: $(nginx -v 2>&1 | sed 's|.*/||')"; else ok "Nginx already installed ($(nginx -v 2>&1 | sed 's|.*/||')) — global config untouched"; fi

# The vhost template uses limit_req zone=general: define the zones only if nobody did
if ! grep -rqs 'zone=general:' /etc/nginx/nginx.conf /etc/nginx/conf.d/; then
    cat > /etc/nginx/conf.d/rate-limiting.conf <<'RATE_EOF'
# Generado por versaDeploy — usadas por los vhosts (limit_req zone=general)
limit_req_zone $binary_remote_addr zone=general:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=api:10m rate=30r/s;
limit_req_zone $binary_remote_addr zone=login:10m rate=20r/m;
RATE_EOF
    ok "Rate limiting zones added: /etc/nginx/conf.d/rate-limiting.conf"
fi

if [ -n "${WEB_FRESH:-}" ]; then
    systemctl enable nginx >/dev/null 2>&1
    systemctl start nginx
fi
install_site_config
service_active nginx || die "nginx is not running — journalctl -u nginx -n 30"
ok "Nginx running"
