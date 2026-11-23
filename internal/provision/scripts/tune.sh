# provision.tuning on a server in use: only this project's files and role, unless
# server_wide says otherwise. Nothing restarts unless named in --restart.
wants_restart() { case " ${TUNE_RESTART:-} " in *" $1 "*) return 0 ;; *) return 1 ;; esac; }

if [ -f Caddyfile ]; then
    cfg="/etc/frankenphp/${PROJECT_NAME}.Caddyfile"
    /usr/local/bin/frankenphp validate --config Caddyfile --adapter caddyfile >/dev/null 2>&1 \
        || die "invalid FrankenPHP config: $(/usr/local/bin/frankenphp validate --config Caddyfile --adapter caddyfile 2>&1 | tail -1)"
    if ! cmp -s Caddyfile "$cfg"; then
        install -m 644 Caddyfile "$cfg"
        if wants_restart "$APP_SERVICE"; then
            systemctl restart "$APP_SERVICE" && ok "${APP_SERVICE} restarted with the new threads"
        else
            warn "FrankenPHP config updated: applies on the next restart (versa tune --apply --restart ${APP_SERVICE})"
        fi
    else
        ok "FrankenPHP config already up to date"
        if wants_restart "$APP_SERVICE"; then systemctl restart "$APP_SERVICE" && ok "${APP_SERVICE} restarted"; fi
    fi
fi

[ -z "${WEB_SERVER:-}" ] || install_site_config

case "${DATABASE:-}" in
postgres)
    if service_active postgresql; then
        sudo -u postgres psql -v ON_ERROR_STOP=1 -q \
            -c "ALTER ROLE ${DB_USER} SET idle_in_transaction_session_timeout = '60s'" \
            -c "ALTER ROLE ${DB_USER} SET log_min_duration_statement = '500ms'"
        ok "PostgreSQL role ${DB_USER}: limits apply to new connections"
        if [ "${TUNE_SERVER_WIDE:-false}" = true ]; then
            sudo -u postgres psql -v ON_ERROR_STOP=1 -q \
                -c "ALTER SYSTEM SET shared_buffers = '${TUNE_PG_SHARED}MB'" \
                -c "ALTER SYSTEM SET effective_cache_size = '${TUNE_PG_CACHE}MB'" \
                -c "ALTER SYSTEM SET maintenance_work_mem = '${TUNE_PG_MAINT}MB'" \
                -c "SELECT pg_reload_conf()" >/dev/null
            sleep 1
            if [ -n "$(sudo -u postgres psql -tAc "SELECT name FROM pg_settings WHERE pending_restart")" ]; then
                if wants_restart postgresql; then
                    systemctl restart postgresql && ok "PostgreSQL restarted"
                else
                    warn "shared_buffers applies on the next restart (versa tune --apply --restart postgresql)"
                fi
            fi
        fi
    fi
    ;;
mariadb)
    if [ "${TUNE_SERVER_WIDE:-false}" = true ]; then
        if [ -d /etc/mysql/mariadb.conf.d ]; then cnf=/etc/mysql/mariadb.conf.d/99-versa-tuning.cnf; else cnf=/etc/my.cnf.d/99-versa-tuning.cnf; fi
        printf '# Generado por versaDeploy (provision.tuning)\n[mysqld]\ninnodb_buffer_pool_size = %sM\n' "$TUNE_INNODB" > "$cnf"
        if mysql --protocol=socket -u root -e "SET GLOBAL innodb_buffer_pool_size = ${TUNE_INNODB} * 1024 * 1024" 2>/dev/null; then
            ok "InnoDB buffer pool: ${TUNE_INNODB} MB"
        else
            warn "InnoDB buffer pool applies on the next restart"
        fi
        if wants_restart mariadb; then systemctl restart mariadb && ok "MariaDB restarted"; fi
    fi
    ;;
esac
