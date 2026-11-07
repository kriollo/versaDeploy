id "$DEPLOY_USER" >/dev/null 2>&1 || die "user ${DEPLOY_USER} does not exist (select 'Deploy user')"
# Root must not follow a symlink the deploy user planted in the path: chown/chmod
# through it would hand them any directory on the server.
[ "$(readlink -m -- "$REMOTE_PATH")" = "$REMOTE_PATH" ] || die "${REMOTE_PATH} goes through a symlink: use the real path"
mkdir -p "$REMOTE_PATH"
(
    # Work on the opened directory (.) so a swap after the check can't redirect it.
    cd -P "$REMOTE_PATH" && [ "$(pwd -P)" = "$REMOTE_PATH" ] || die "${REMOTE_PATH} goes through a symlink: use the real path"
    # Only the base dir: versaDeploy creates releases/, shared/ and current itself
    chown "${DEPLOY_USER}:www-data" .
    # 2755: www-data (PHP-FPM) reads but can't rename .versa/, current or backups/
    chmod 2755 .
)
# SELinux: the web server and PHP-FPM read the releases and write logs, cache and
# uploads (same as the group-writable dirs below it); files created later inherit it
selinux_label httpd_sys_rw_content_t "${REMOTE_PATH}(/.*)?" "$REMOTE_PATH"
ok "${REMOTE_PATH} ready (${DEPLOY_USER}:www-data, 2755)"
