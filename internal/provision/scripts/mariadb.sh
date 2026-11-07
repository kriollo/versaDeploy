# An existing MySQL or MariaDB server is used as is: config untouched, no restart.
# Installing mariadb-server over MySQL would replace it.
if package_installed mysql-server || { command -v mysqld >/dev/null && ! command -v mariadbd >/dev/null; }; then
    unit=mysql
    ok "MySQL already installed ($(mysql --version | sed 's/^mysql *//')) — using it, config untouched"
elif package_installed mariadb-server; then
    unit=mariadb
    ok "MariaDB already installed — config untouched"
else
    unit=mariadb
    pkg_install mariadb-server
    if [ -d /etc/mysql/mariadb.conf.d ]; then
        cnf=/etc/mysql/mariadb.conf.d/99-versadeploy.cnf
    else
        mkdir -p /etc/my.cnf.d
        cnf=/etc/my.cnf.d/99-versadeploy.cnf
    fi
    cat > "$cnf" <<'MYCNF'
# Generado por versaDeploy — solo conexiones locales
[mysqld]
bind-address = 127.0.0.1
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
MYCNF
    systemctl enable mariadb >/dev/null 2>&1
    systemctl restart mariadb
    ok "MariaDB installed (127.0.0.1, utf8mb4)"
fi
service_active "$unit" || die "$unit is not running — journalctl -u $unit -n 30"

# The password reaches the client through stdin (printf is a builtin), never argv,
# so other local users can't read it from the process list. The SET pins the
# escaping mode pass_sql assumes.
pass_sql="${DB_PASS//\\/\\\\}"
pass_sql="${pass_sql//\'/\\\'}"
mysqlc() { mysql --protocol=socket -u root "$@"; }
mysqlc -e "CREATE DATABASE IF NOT EXISTS \`${DB_NAME}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
if [ "$(mysqlc -N -e "SELECT COUNT(*) FROM mysql.user WHERE user='${DB_USER}' AND host='localhost'")" != 0 ]; then
    if [ "${RESET_DB_PASSWORD:-false}" = true ]; then
        printf '%s\n' "SET SESSION sql_mode=REPLACE(@@sql_mode,'NO_BACKSLASH_ESCAPES',''); ALTER USER '${DB_USER}'@'localhost' IDENTIFIED BY '${pass_sql}';" | mysqlc
        warn "DB user ${DB_USER}: password CHANGED (explicit reset) — update the app's .env"
    else
        ok "DB user ${DB_USER} exists — password kept (reset it explicitly from the Services view)"
    fi
else
    [ -n "$DB_PASS" ] || die "db_password is required to create ${DB_USER}"
    printf '%s\n' "SET SESSION sql_mode=REPLACE(@@sql_mode,'NO_BACKSLASH_ESCAPES',''); CREATE USER '${DB_USER}'@'localhost' IDENTIFIED BY '${pass_sql}';" | mysqlc
    ok "DB user ${DB_USER} created"
fi
mysqlc -e "GRANT ALL PRIVILEGES ON \`${DB_NAME}\`.* TO '${DB_USER}'@'localhost'; FLUSH PRIVILEGES;"
ok "${unit} ready — ${DB_NAME} / ${DB_USER} @ localhost:3306"
