id "$DEPLOY_USER" >/dev/null 2>&1 || die "user ${DEPLOY_USER} does not exist (select 'Deploy user')"
mkdir -p "$REMOTE_PATH"
# Only the base dir: versaDeploy creates releases/, shared/ and current itself
chown "${DEPLOY_USER}:www-data" "$REMOTE_PATH"
# 2755: www-data (PHP-FPM) reads but can't rename .versa/, current or backups/
chmod 2755 "$REMOTE_PATH"
ok "${REMOTE_PATH} ready (${DEPLOY_USER}:www-data, 2755)"
