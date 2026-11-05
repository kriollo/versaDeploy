if id "$DEPLOY_USER" >/dev/null 2>&1; then
    ok "User ${DEPLOY_USER} exists"
else
    adduser --disabled-password --gecos "" "$DEPLOY_USER" >/dev/null
    ok "User ${DEPLOY_USER} created"
fi
getent group www-data >/dev/null || groupadd www-data
usermod -aG www-data "$DEPLOY_USER"
home="$(getent passwd "$DEPLOY_USER" | cut -d: -f6)"
# as the user, not root: an existing user controls ~/.ssh (a symlink there must not make
# root chmod/chown another path)
runuser -u "$DEPLOY_USER" -- sh -c 'set -e
    umask 077
    mkdir -p "$1/.ssh"
    touch "$1/.ssh/authorized_keys"
    chmod 700 "$1/.ssh"
    chmod 600 "$1/.ssh/authorized_keys"
    if [ -n "$2" ] && ! grep -qxF "$2" "$1/.ssh/authorized_keys"; then
        echo "$2" >> "$1/.ssh/authorized_keys" && echo "[OK]    Public key added to $1/.ssh/authorized_keys"
    fi' _ "$home" "$PUBKEY"
ok "SSH access ready for ${DEPLOY_USER}"
