pg_fresh=""
package_installed postgresql || pg_fresh=1
apt_install postgresql postgresql-contrib
if [ -n "$pg_fresh" ]; then
    systemctl enable postgresql >/dev/null 2>&1
    systemctl start postgresql
fi
service_active postgresql || die "PostgreSQL is not running — journalctl -u postgresql -n 30"
pg_major="$(psql --version | awk '{print $3}' | cut -d. -f1)"
pg_dir="/etc/postgresql/${pg_major}/main"

if [ -n "$pg_fresh" ] && [ -f "$pg_dir/postgresql.conf" ]; then
    # tuning only on a server installed by this run; an existing one keeps its settings
    sed -i "s|^#*shared_buffers\s*=.*|shared_buffers = 256MB|" "$pg_dir/postgresql.conf"
    sed -i "s|^#*max_connections\s*=.*|max_connections = 100|" "$pg_dir/postgresql.conf"
    if grep -q "^#*password_encryption" "$pg_dir/postgresql.conf"; then
        sed -i "s|^#*password_encryption\s*=.*|password_encryption = scram-sha-256|" "$pg_dir/postgresql.conf"
    else
        echo "password_encryption = scram-sha-256" >> "$pg_dir/postgresql.conf"
    fi
    systemctl restart postgresql
    ok "PostgreSQL ${pg_major} installed and tuned (shared_buffers=256MB, max_connections=100, scram-sha-256)"
else
    ok "PostgreSQL ${pg_major} already installed — postgresql.conf untouched, no restart"
fi

pass_sql="${DB_PASS//\'/\'\'}"
psqlc() { sudo -u postgres psql -v ON_ERROR_STOP=1 -q "$@"; }
if [ "$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'")" = 1 ]; then
    if [ "${RESET_DB_PASSWORD:-false}" = true ]; then
        psqlc -c "ALTER USER ${DB_USER} WITH ENCRYPTED PASSWORD '${pass_sql}';"
        warn "DB user ${DB_USER}: password CHANGED (explicit reset) — update the app's .env"
    else
        ok "DB user ${DB_USER} exists — password kept (reset it explicitly from the Services view)"
    fi
else
    [ -n "$DB_PASS" ] || die "db_password is required to create ${DB_USER}"
    psqlc -c "CREATE USER ${DB_USER} WITH ENCRYPTED PASSWORD '${pass_sql}';"
    ok "DB user ${DB_USER} created"
fi
if [ "$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'")" = 1 ]; then
    ok "Database ${DB_NAME} exists — left as is"
else
    psqlc -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER} ENCODING 'UTF8';"
    ok "Database ${DB_NAME} created"
fi
psqlc -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"
psqlc -d "$DB_NAME" -c "GRANT ALL ON SCHEMA public TO ${DB_USER};"
psqlc -d "$DB_NAME" -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER};"
ok "Privileges ensured for ${DB_USER} on ${DB_NAME}"

hba="$pg_dir/pg_hba.conf"
if [ -f "$hba" ] && ! grep -q "^host\s\+${DB_NAME}\s\+${DB_USER}\s" "$hba"; then
    # same method the server hashes passwords with (an md5 server rejects scram rules)
    method="$(sudo -u postgres psql -tAc 'SHOW password_encryption' | tr -d ' ')"
    [ "$method" = md5 ] || method=scram-sha-256
    rule="host    ${DB_NAME}    ${DB_USER}    127.0.0.1/32    ${method}"
    if grep -q "^# IPv4 local connections:" "$hba"; then
        sed -i "/^# IPv4 local connections:/i # versaDeploy — ${PROJECT_NAME}\n${rule}" "$hba"
    else
        printf '\n# versaDeploy — %s\n%s\n' "$PROJECT_NAME" "$rule" >> "$hba"
    fi
    # pg_hba.conf only needs a reload: open connections stay up
    systemctl reload postgresql
    ok "pg_hba.conf: ${method} rule for ${DB_USER}@${DB_NAME} (reloaded)"
fi
ok "PostgreSQL ready — ${DB_NAME} / ${DB_USER} @ 127.0.0.1:5432"
