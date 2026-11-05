if id "$DEPLOY_USER" >/dev/null 2>&1; then
    ok "User ${DEPLOY_USER} exists"
else
    adduser --disabled-password --gecos "" "$DEPLOY_USER" >/dev/null
    ok "User ${DEPLOY_USER} created"
fi
getent group www-data >/dev/null || groupadd www-data
usermod -aG www-data "$DEPLOY_USER"
home="$(getent passwd "$DEPLOY_USER" | cut -d: -f6)"
mkdir -p "$home/.ssh"
touch "$home/.ssh/authorized_keys"
if [ -n "$PUBKEY" ] && ! grep -qxF "$PUBKEY" "$home/.ssh/authorized_keys"; then
    echo "$PUBKEY" >> "$home/.ssh/authorized_keys"
    ok "Public key added to ${home}/.ssh/authorized_keys"
fi
chmod 700 "$home/.ssh"
chmod 600 "$home/.ssh/authorized_keys"
chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "$home/.ssh"
ok "SSH access ready for ${DEPLOY_USER}"
