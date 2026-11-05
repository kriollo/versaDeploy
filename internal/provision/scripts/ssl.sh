[ -n "$WEB_SERVER" ] || die "select a web server for SSL"
plugin="--$WEB_SERVER"
apt_install certbot "python3-certbot-${WEB_SERVER}"
ok "Certbot installed"

server_ip="$(curl -4 -s --max-time 8 https://api64.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"
domain_ip="$(getent hosts "$DOMAIN" | awk '{print $1}' | head -1 || true)"
if [ -z "$domain_ip" ] || [ "$server_ip" != "$domain_ip" ]; then
    warn "${DOMAIN} resolves to '${domain_ip:-nothing}', this server is '${server_ip:-unknown}'"
    warn "SSL skipped: Let's Encrypt would fail. When DNS is ready run SSL again, or:"
    warn "  certbot certonly ${plugin} -d ${DOMAIN} --non-interactive --agree-tos -m ${ADMIN_EMAIL}"
else
    domains=(-d "$DOMAIN")
    # aliases join the certificate only when they resolve here, or Let's Encrypt fails
    for a in $ALIASES; do
        if [ "$(getent hosts "$a" | awk '{print $1}' | head -1)" = "$server_ip" ]; then
            domains+=(-d "$a")
        else
            warn "Alias $a does not resolve to this server — left out of the certificate"
        fi
    done
    # certonly: certbot only obtains the cert; the vhost stays ours (vhost-ssl.conf)
    certbot certonly "$plugin" "${domains[@]}" --non-interactive --agree-tos -m "$ADMIN_EMAIL" --keep-until-expiring --expand
    ensure_ssl_includes
    install_vhost vhost-ssl.conf
    ok "SSL active for ${DOMAIN} (renewal: certbot.timer)"
fi
