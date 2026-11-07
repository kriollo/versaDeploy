# RHEL family: firewalld is the system firewall (the nginx step opens 80/443 in it);
# ufw next to it would fight over the same rules
[ "$PKG_MGR" = apt ] || die "UFW is for Debian/Ubuntu — on this server use firewalld (the web server step opens 80/443)"
# SSH_PORT is the port this very session uses, so the rules can't lock us out
rules() {
    ufw allow "${SSH_PORT}/tcp" comment "SSH" >/dev/null
    ufw allow 80/tcp comment "HTTP" >/dev/null
    ufw allow 443/tcp comment "HTTPS" >/dev/null
}
if package_installed ufw && ufw status | grep -q '^Status: active'; then
    # an active firewall keeps its policy and rules: only the ports the site needs are added
    rules
    ok "UFW already active — policy untouched, allowed ${SSH_PORT}/tcp, 80/tcp, 443/tcp"
else
    apt_install ufw
    ufw --force default deny incoming >/dev/null
    ufw --force default allow outgoing >/dev/null
    rules
    ufw --force enable >/dev/null
    ok "UFW enabled: deny incoming except ${SSH_PORT}/tcp, 80/tcp, 443/tcp"
fi
ufw status verbose || true
