id "$DEPLOY_USER" >/dev/null 2>&1 || die "user ${DEPLOY_USER} does not exist (select 'Deploy user')"
sc="$(command -v systemctl)"
cmds=("$sc daemon-reload")
add() { for a in "$@"; do cmds+=("$sc $a"); done; }
case "$WEB_SERVER" in
    nginx)  add "reload nginx" "restart nginx" ;;
    apache) add "reload apache2" "restart apache2" ;;
esac
[ -n "$PHP_VERSION" ] && add "reload php${PHP_VERSION}-fpm" "restart php${PHP_VERSION}-fpm"
line="$(printf '%s, ' "${cmds[@]}")"
tmp="$(mktemp)"
cat > "$tmp" <<SUDOERS_EOF
# Sudoers para ${DEPLOY_USER} — proyecto ${PROJECT_NAME} (generado por versaDeploy)
${DEPLOY_USER} ALL=(ALL) NOPASSWD: ${line%, }
SUDOERS_EOF
# deploy.yml services: the rules `versa service <env> sudoers` prints, so deploys can
# install and restart them as the deploy user
if [ -f services-sudoers.sh ]; then
    sh services-sudoers.sh | grep -v '^#' >> "$tmp"
    ok "Rules for deploy.yml services added"
fi
# One file per project so several apps on a server keep their rules. sudo skips files
# whose name contains '.', hence the tr.
file="/etc/sudoers.d/$(printf '%s-%s' "$DEPLOY_USER" "$PROJECT_NAME" | tr -c 'A-Za-z0-9_-' '_')"
if visudo -c -f "$tmp" >/dev/null; then
    install -m 440 "$tmp" "$file"
    rm -f "$tmp"
    ok "Sudoers validated: $file"
else
    rm -f "$tmp"
    die "sudoers syntax error — not applied"
fi
