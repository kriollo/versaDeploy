info "Installing essential tools..."
apt_install software-properties-common curl wget git unzip zip nano make build-essential \
    ca-certificates gnupg apt-transport-https lsb-release openssl htop ufw gettext-base
ok "Essential packages installed (system upgrade is done from the Services view: U/S)"
