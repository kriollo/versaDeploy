# Never install an alternative web server: both want ports 80/443
if package_installed apache2; then
    die "Apache is already installed — nginx not installed. Use the Apache step (web_server: apache)"
fi
package_installed nginx || package_installed nginx-core || WEB_FRESH=1
apt_install nginx libnginx-mod-http-headers-more-filter
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
