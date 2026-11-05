[ "$DATABASE" = postgres ] || [ "$DATABASE" = mariadb ] || die "backups need PostgreSQL or MariaDB"
id "$DEPLOY_USER" >/dev/null 2>&1 || die "user ${DEPLOY_USER} does not exist (select 'Deploy user')"
unit="versa-backup-${PROJECT_NAME}"
bin="/usr/local/sbin/${unit}"

# The dump script carries its settings: the timer runs it without this bundle.
{
    echo '#!/usr/bin/env bash'
    echo "# Generado por versaDeploy — backup diario de ${DB_NAME} (${DATABASE})"
    printf 'DATABASE=%q DB_NAME=%q DIR=%q OWNER=%q KEEP=%q\n' "$DATABASE" "$DB_NAME" "${REMOTE_PATH}/backups" "$DEPLOY_USER" "$BACKUP_KEEP"
    cat <<'DUMP'
set -euo pipefail
# DIR lives in the deploy user's tree: every file operation runs as OWNER, so a symlink
# planted there can't make root write, chown or delete another path
as() { runuser -u "$OWNER" -- "$@"; }
as sh -c 'umask 077; mkdir -p "$1" && chmod 700 "$1"' _ "$DIR"
f="$DIR/${DB_NAME}-$(date +%Y%m%d-%H%M%S).sql.gz"
if [ "$DATABASE" = postgres ]; then
    sudo -u postgres pg_dump --no-owner "$DB_NAME"
else
    mysqldump --single-transaction --routines --triggers "$DB_NAME"
fi | gzip -6 | as sh -c 'umask 077; cat > "$1"' _ "$f.tmp"
as mv "$f.tmp" "$f"
# keep the newest $KEEP dumps
as sh -c 'ls -1t "$1/$2"-*.sql.gz | tail -n +"$(($3 + 1))" | xargs -r rm -f' _ "$DIR" "$DB_NAME" "$KEEP"
echo "backup ok: $f ($(du -h "$f" | cut -f1)), keeping $KEEP"
DUMP
} > "$bin"
chmod 700 "$bin"

cat > "/etc/systemd/system/${unit}.service" <<UNIT
[Unit]
Description=versaDeploy DB backup — ${PROJECT_NAME}

[Service]
Type=oneshot
ExecStart=${bin}
UNIT
cat > "/etc/systemd/system/${unit}.timer" <<UNIT
[Unit]
Description=Daily DB backup — ${PROJECT_NAME}

[Timer]
OnCalendar=daily
RandomizedDelaySec=30m
Persistent=true

[Install]
WantedBy=timers.target
UNIT
systemctl daemon-reload
systemctl enable --now "${unit}.timer" >/dev/null 2>&1
ok "Timer ${unit}.timer active (daily, keep ${BACKUP_KEEP})"
systemctl start "${unit}.service" && journalctl -u "${unit}.service" -n 1 --no-pager -o cat || warn "First backup failed — journalctl -u ${unit}.service"
