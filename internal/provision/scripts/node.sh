major="${NODE_MAJOR:-22}"
current="$(node -v 2>/dev/null | sed 's/^v//; s/\..*//' || true)"
if [ "$current" = "$major" ]; then
    ok "Node.js ${major} already installed ($(node -v))"
elif [ -n "$current" ]; then
    # never replace an installed Node with another major version
    die "Node.js $(node -v) is already installed — Node ${major} not installed. Set node_major: \"${current}\""
else
    info "Installing Node.js ${major} from NodeSource..."
    apt_install ca-certificates curl gnupg
    mkdir -p /etc/apt/keyrings
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor --yes -o /etc/apt/keyrings/nodesource.gpg
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${major}.x nodistro main" > /etc/apt/sources.list.d/nodesource.list
    apt-get update -qq
    apt_install nodejs
    ok "Node.js installed: $(node -v)"
fi
