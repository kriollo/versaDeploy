# Never install an alternative web server: both want ports 80/443
if package_installed nginx || package_installed nginx-core; then
    die "Nginx is already installed — Apache not installed. Use the Nginx step (web_server: nginx)"
fi
package_installed apache2 || WEB_FRESH=1
apt_install apache2
if [ -n "${WEB_FRESH:-}" ]; then ok "Apache2 installed"; else ok "Apache2 already installed — global config untouched"; fi

# Modules the vhost needs; enabling one is additive and the vhost reload loads it
enabled=""
for mod in ssl rewrite headers expires deflate proxy proxy_fcgi proxy_http proxy_wstunnel http2 setenvif; do
    if [ ! -e "/etc/apache2/mods-enabled/${mod}.load" ]; then
        a2enmod "$mod" >/dev/null 2>&1 && enabled="$enabled $mod" || warn "Could not enable module $mod"
    fi
done
[ -n "$enabled" ] && ok "Apache2 modules enabled:$enabled"

if [ -n "${WEB_FRESH:-}" ]; then
    systemctl enable apache2 >/dev/null 2>&1
    systemctl start apache2
fi
install_site_config
service_active apache2 || die "apache2 is not running — journalctl -u apache2 -n 30"
ok "Apache2 running"
