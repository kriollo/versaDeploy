# RHEL family: Node from the AppStream module stream nodejs:N
if [ "$PKG_MGR" != apt ]; then
    major="${NODE_MAJOR:-22}"
    current="$(node -v 2>/dev/null | sed 's/^v//; s/\..*//' || true)"
    if [ "$current" = "$major" ]; then
        ok "Node.js ${major} already installed ($(node -v))"
        return 0
    elif [ -n "$current" ]; then
        die "Node.js $(node -v) is already installed — Node ${major} not installed. Set node_major: \"${current}\""
    fi
    dnf module reset -y -q nodejs >/dev/null 2>&1 || true
    dnf module enable -y -q "nodejs:${major}" >/dev/null 2>&1 || true
    cand="$(dnf_candidate nodejs | cut -d. -f1)"
    [ "$cand" = "$major" ] || die "Node.js ${major} is not available for ${OS_ID} (its repos offer ${cand:-none}) — set node_major: \"${cand}\""
    pkg_install nodejs npm
    ok "Node.js installed: $(node -v)"
    return 0
fi
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
