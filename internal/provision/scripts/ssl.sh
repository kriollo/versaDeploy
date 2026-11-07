[ -n "$WEB_SERVER" ] || die "select a web server for SSL"
if [ "${SSL_SELF_SIGNED:-false}" = true ]; then
    # staging/LAN: no public DNS, so no Let's Encrypt. Kept across runs (renew: delete it).
    d="$CERT_DIR"
    if [ ! -f "$d/fullchain.pem" ]; then
        san="DNS:${DOMAIN}"
        for a in $ALIASES; do san="${san},DNS:${a}"; done
        mkdir -p "$d"
        openssl req -x509 -newkey rsa:2048 -nodes -days 825 -subj "/CN=${DOMAIN}" -addext "subjectAltName=${san}" \
            -keyout "$d/privkey.pem" -out "$d/fullchain.pem" 2>/dev/null
        chmod 600 "$d/privkey.pem"
        ok "Self-signed certificate for ${DOMAIN} (${san}), valid 825 days"
    fi
    # trusted on the server itself: the app's own HTTPS probes (app:check:deploy) verify it
    if [ "$PKG_MGR" = apt ]; then
        cp "$d/fullchain.pem" "/usr/local/share/ca-certificates/versa-${DOMAIN}.crt" && update-ca-certificates >/dev/null
    else
        cp "$d/fullchain.pem" "/etc/pki/ca-trust/source/anchors/versa-${DOMAIN}.crt" && update-ca-trust
    fi
    ensure_ssl_includes
    install_vhost vhost-ssl.conf
    warn "Browsers warn about a self-signed certificate: import ${d}/fullchain.pem, or use Let's Encrypt for a public site"
    return 0
fi
plugin="--$WEB_SERVER"
# RHEL family: both packages come from EPEL
pkg_install certbot "python3-certbot-${WEB_SERVER}"
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
