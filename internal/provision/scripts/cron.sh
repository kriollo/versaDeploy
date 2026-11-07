# provision.cron: one cron.d file per project, rewritten whole (removed jobs go away)
tmp="${CRON_PATH}.versa-tmp"
printf '%s' "$CRON_FILE" > "$tmp"
chmod 644 "$tmp"
mv -f "$tmp" "$CRON_PATH"
cron_unit=cron; [ "$PKG_MGR" = apt ] || cron_unit=crond
service_active "$cron_unit" || warn "${cron_unit} is not running — select Base packages"
ok "Cron jobs installed: ${CRON_PATH} ($(grep -c "^[@0-9*]" "$CRON_PATH") job(s), as ${DEPLOY_USER})"
