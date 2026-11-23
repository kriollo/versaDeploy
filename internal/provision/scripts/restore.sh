set -eu
as() { runuser -u "$OWNER" -- "$@"; }
as sh -c 'gzip -t < "$1"' _ "$FILE" || { echo "[ERROR] corrupt backup: $FILE"; exit 1; }
tmp="${DB_NAME}_restore_$(date +%s)"
cleanup() {
    [ "$KEEP" = true ] && return 0
    if [ "$DATABASE" = postgres ]; then
        sudo -u postgres psql -q -c "DROP DATABASE IF EXISTS ${tmp}" || true
        [ -z "${made_role:-}" ] || sudo -u postgres psql -q -c "DROP ROLE IF EXISTS \"${OWNER}\"" || true
    else
        mysql --protocol=socket -u root -e "DROP DATABASE IF EXISTS \`${tmp}\`" || true
        [ -z "${made_role:-}" ] || mysql --protocol=socket -u root -e "DROP USER IF EXISTS '${OWNER}'@'localhost'" || true
    fi
}
trap cleanup EXIT
if [ "$DATABASE" = postgres ]; then
    if [ "$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${OWNER}'")" != 1 ]; then
        sudo -u postgres psql -q -c "CREATE ROLE \"${OWNER}\" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE"
        made_role=1
    fi
    [ "$(sudo -u postgres psql -tAc "SELECT rolsuper FROM pg_roles WHERE rolname='${OWNER}'")" = f ] || { echo "[ERROR] role ${OWNER} is a superuser"; exit 1; }
    sudo -u postgres createdb -O "$OWNER" "$tmp"
    as sh -c 'gzip -dc < "$1" | grep -vE "^(ALTER DEFAULT PRIVILEGES|GRANT |REVOKE )" | psql -q -v ON_ERROR_STOP=1 --single-transaction -d "$2" >/dev/null' _ "$FILE" "$tmp"
    tables="$(sudo -u postgres psql -d "$tmp" -tAc "SELECT quote_ident(tablename) FROM pg_tables WHERE schemaname='public' ORDER BY 1")"
    count() { sudo -u postgres psql -d "$1" -tAc "SELECT count(*) FROM public.$2" 2>/dev/null || echo '-'; }
else
    mysql --protocol=socket -u root -e "CREATE DATABASE \`${tmp}\`"
    if [ "$(mysql --protocol=socket -u root -N -e "SELECT COUNT(*) FROM mysql.user WHERE user='${OWNER}' AND host='localhost'")" = 0 ]; then
        mysql --protocol=socket -u root -e "CREATE USER '${OWNER}'@'localhost' IDENTIFIED VIA unix_socket" 2>/dev/null \
            || mysql --protocol=socket -u root -e "CREATE USER '${OWNER}'@'localhost' IDENTIFIED WITH auth_socket"
        made_role=1
    fi
    mysql --protocol=socket -u root -e "GRANT ALL PRIVILEGES ON \`${tmp}\`.* TO '${OWNER}'@'localhost'"
    as sh -c 'gzip -dc < "$1" | mysql --protocol=socket "$2"' _ "$FILE" "$tmp"
    tables="$(mysql --protocol=socket -u root -N -e "SELECT table_name FROM information_schema.tables WHERE table_schema='${tmp}' AND table_type='BASE TABLE' ORDER BY 1")"
    count() { mysql --protocol=socket -u root -N -e "SELECT COUNT(*) FROM \`$1\`.\`$2\`" 2>/dev/null || echo '-'; }
fi
n=0; rows=0; diff=0
for t in $tables; do
    a="$(count "$tmp" "$t")"; b="$(count "$DB_NAME" "$t")"
    n=$((n + 1)); [ "$a" = '-' ] || rows=$((rows + a))
    if [ "$a" != "$b" ]; then diff=$((diff + 1)); printf '  %-40s backup %s · live %s\n' "$t" "$a" "$b"; fi
done
echo "restore ok: ${n} tables, ${rows} rows in ${tmp}; ${diff} table(s) differ from live"
[ "$KEEP" != true ] || echo "kept: ${tmp}"
