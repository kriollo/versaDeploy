info "Installing essential tools..."
apt_install software-properties-common curl wget git unzip zip nano make build-essential \
    ca-certificates gnupg apt-transport-https lsb-release openssl htop ufw gettext-base cron
# app schedulers (versaWYS app:schedule:run) need cron running; a running one is left as is
service_active cron || systemctl enable --now cron >/dev/null 2>&1 || warn "cron could not be started"
ok "Essential packages installed (system upgrade is done from the Services view: U/S)"
