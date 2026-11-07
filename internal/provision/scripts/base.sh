info "Installing essential tools..."
if [ "$PKG_MGR" = apt ]; then
    apt_install software-properties-common curl wget git unzip zip nano make build-essential \
        ca-certificates gnupg apt-transport-https lsb-release openssl htop ufw gettext-base cron
else
    # curl-minimal ships with the OS (installing curl conflicts with it). EPEL (htop,
    # nginx headers-more) is in Rocky/Alma's repos; RHEL needs it enabled by hand.
    pkg_install wget git unzip zip nano make gcc gcc-c++ ca-certificates gnupg2 openssl sudo cronie diffutils
    pkg_install epel-release 2>/dev/null || warn "EPEL not available from the enabled repos"
    pkg_install htop 2>/dev/null || true
fi
# provision.packages: what the app needs from the OS (OCR, image tools...)
if [ -n "${PACKAGES:-}" ]; then
    # shellcheck disable=SC2086 # one word per package (names are validated)
    pkg_install $PACKAGES
    ok "Extra packages: ${PACKAGES}"
fi
# app schedulers (versaWYS app:schedule:run) need cron running; a running one is left as is
cron_unit=cron; [ "$PKG_MGR" = apt ] || cron_unit=crond
service_active "$cron_unit" || systemctl enable --now "$cron_unit" >/dev/null 2>&1 || warn "cron could not be started"
ok "Essential packages installed (system upgrade is done from the Services view: U/S)"
