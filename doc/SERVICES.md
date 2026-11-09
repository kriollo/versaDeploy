# 🧩 Services: server setup, updates and backups

The **Services** view of the TUI (`versa`, tab *Services*) turns versaDeploy into a small, agentless server panel: it detects what the server runs, installs the stack your app needs, applies updates, warns about security issues, shows logs and schedules database backups. Everything goes over SSH — nothing is installed permanently on the server besides the services you pick.

> [!NOTE]
> Detection works on any Linux server. **Installing and backups need Debian/Ubuntu (apt) or RHEL/Rocky/Alma 8+ (dnf)**; upgrades from the view (`U`/`S`/`Ctrl+R`) are apt only. On other systems the view is read-only. RHEL family specifics: [§5.1](#51-rhel-family-rockyalma).

---

## 1. Requirements

| What | Why |
| :--- | :-- |
| An environment in `deploy.yml` with `ssh` and `remote_path` | The view works on the active environment. |
| `provision.ssh` with a root (or passwordless-sudo) user | Installs and upgrades run as root. Detection uses the normal deploy connection. |
| The server in your `known_hosts` | versa refuses unknown host keys: `ssh-keyscan -p PORT HOST >> ~/.ssh/known_hosts`. |
| `<ssh.key_path>.pub` next to your private key | It becomes the deploy user's `authorized_keys` when versa creates that user. |

Minimal block:

```yaml
environments:
  production:
    ssh: { host: 203.0.113.10, user: deploy, key_path: ~/.ssh/id_ed25519 }
    remote_path: /var/www/myapp
    provision:
      ssh: { user: root }          # host, port and key are inherited from ssh
      domain: myapp.com
```

---

## 2. From a fresh server to the first deploy

1. Point the domain's DNS to the server and run `ssh-keyscan` (see above).
2. Run `versa` and open **Services** (← / → or click the tab). The deploy user doesn't exist yet: the view detects through the root connection and shows *"Deploy user … does not exist yet"*.
3. Check the preselected services (they come from your app's backend, see §4), adjust the **Parameters** and press **`I`** → **`Y`**. The log streams live.
4. Or press **`P`** (*prepare for deploy*) to create only the deploy user (with your public key), its sudoers rules and the project directory.
5. Press **`w`** to save the `provision:` block and **`a`** to add the suggested `services_reload` commands to `deploy.yml`. Commit `deploy.yml`.
6. Go to **Operations**, enable **Initial deploy** and press **`D`**.

---

## 3. Keys

| Key | Action |
| :-- | :----- |
| `↑` `↓` | Move between services and parameters |
| `Space` | Select / unselect a service (web server and database groups are exclusive) |
| `Enter` | On a service: select. On a parameter: edit it. On a version: next version |
| `e` | Type a parameter value (also for versions) |
| `I` | Install / configure the selection (confirmation first) |
| `P` | Prepare the server for deploy: deploy user + key, sudoers, directory |
| `U` | Upgrade the service under the cursor (only its packages) |
| `S` | Apply every pending **security** update |
| `Ctrl+R` | Refresh the package index (`apt-get update`) |
| `L` | Show the last log lines of the service under the cursor (journal + log files) |
| `B` | Run a database backup now |
| `X` | Reset the database user's password (explicit, see §6) |
| `w` | Save the parameters as `provision:` in `deploy.yml` (comments are kept) |
| `a` | Add the missing `services_reload` commands to `deploy.yml` |
| `F5` | Detect again |
| `Esc` | Close the log / cancel a confirmation |

---

## 4. What it detects

One SSH round-trip (POSIX shell, works on old servers too) reports:

- **Services** — state (`● active`, `● stopped`, `○ absent`) and version of nginx, Apache, PHP-FPM, PostgreSQL, MariaDB/MySQL, Redis, Certbot, Python, Node.js, UFW, the app's service (from `services`), the deploy user, its sudoers, the project directory and the backup timer.
- **Pending updates** per service (`↑ N update(s)`), in red when they are security updates.
- **Security banner** (also on the Dashboard): pending security updates, *reboot required*, unattended-upgrades off, package index older than 7 days.
- **Sites on this server** — every enabled vhost with its domains and document root. A vhost made by hand or by certbot under another name is matched to your app by its document root.
- **SSL** — the certificate your domain serves (checked from your machine), with its expiry date. Red under 14 days.
- **Backend of the app** — from `deploy.yml` builds, or from the repo files when there are none:

| Backend | Detected from | Preselected |
| :------ | :------------ | :---------- |
| PHP | `builds.php` / `composer.json` | PHP-FPM + web server with FastCGI |
| Python | `builds.python` / `pyproject.toml`, `requirements.txt`, `Pipfile` | Python + reverse proxy to `web_port` + a [`services`](DEPLOY.md#services-services) entry |
| Go | `builds.go` / `go.mod` | reverse proxy + a `services` entry for the binary |
| Node | `package.json` with a `start` script (not a static frontend) | Node.js + reverse proxy + a `services` entry running `npm start` |

Python, Go and Node apps run as deploy.yml [`services`](DEPLOY.md#services-services): **each deploy** restarts them (systemd, OpenRC or SysV) and rolls back if they don't stay up. When the environment has no `services`, the view proposes one (name, `exec` for Node, `PORT` = `proxy_port`), and `w` writes it to `deploy.yml`. The *Sudoers* step, run as root, installs and enables their init files and adds the rules deploys need. It does the same as `versa service <env> setup`. The deploy user can only start/stop/restart them: it never writes a file that root runs.

Services that are already active are never preselected.

---

## 5. Installable services

| Service | What it does |
| :------ | :----------- |
| Base packages | curl, git, unzip, build-essential, ufw, cron (for app schedulers)…, plus `provision.packages` (no system upgrade: use `U`/`S`) |
| PHP-FPM + OPCache | PHP from ondrej (Ubuntu) or sury (Debian), extensions for your DB and Redis, production `php.ini` and OPCache tuned for symlink deploys (`revalidate_path=1`) |
| FrankenPHP | Alternative to PHP-FPM (`frankenphp: true`): the official static build (pinned version, sha256 checked: PHP 8.5 with pdo_pgsql/mysql, redis, intl, gd, opcache, zip…) in `/usr/local/bin`, `php` for hooks and cron (`frankenphp php-cli`, unless a PHP CLI exists), and the project's config `/etc/frankenphp/<project>.Caddyfile`: 127.0.0.1:`proxy_port` (default 8000), production `php_ini`, client IP and `HTTPS` from nginx. The app runs as a `services` entry (`w` proposes it) that deploys **never restart**: OPcache keys scripts by their real path (`opcache.revalidate_path`), so requests run the new release within 2 s of the switch, each one entirely in one release, and the deploy then frees the old release's scripts with `opcache_reset()` (`reload`) through the project's owner-only socket `<remote_path>/.frankenphp-opcache.sock`. No request is dropped (a FrankenPHP restart or `frankenphp reload` drops the ones in flight). The nginx vhost keeps the versaWYS/Laravel rules and serves the static files; only `/index.php` is proxied, with the original URI, and FrankenPHP runs `index.php` for every request. nginx only; x86_64 and arm64 |
| Apache2 / Nginx | Web server + the project's vhost (from the versaWYS templates): security headers, rate limiting (nginx: 10 req/s per IP, burst 50, only on requests that reach the app — static files are never limited, also behind a proxy: Next/Nuxt chunks), caching, sensitive files blocked, `$realpath_root` for PHP. Re-running the step regenerates the vhost (with a backup) without touching the web server |
| SSL | `certbot certonly` for the domain and the aliases that resolve to the server, then the SSL vhost. Skipped with a hint when DNS doesn't point here. With `ssl_self_signed: true` (staging, LAN, Docker): a self-signed certificate in `/etc/ssl/versa/<domain>/`, trusted by the server itself |
| PostgreSQL | Server + database + user, `pg_hba.conf` rule for 127.0.0.1 |
| MariaDB / MySQL | MariaDB, or the MySQL already on the server; database (utf8mb4) + user |
| Redis | Bound to 127.0.0.1, protected mode, `maxmemory 256mb` |
| Dragonfly | Redis-compatible alternative (same protocol and port, multi-threaded): the official release package (pinned version, sha256 checked), bound to 127.0.0.1, `maxmemory 256mb`, `cache_mode` (evicts like `allkeys-lru`), 1 thread (Dragonfly needs 256 MB per thread: raise both together in `/etc/dragonfly/dragonfly.conf`), no version phone-home. x86_64 and arm64 (Debian/Ubuntu); x86_64 on RHEL. Apps keep using phpredis or any Redis client |
| Python 3 | System Python, or another version next to it (Ubuntu: deadsnakes) |
| Node.js | NodeSource LTS |
| Deploy user + SSH key | Creates the user, adds it to `www-data`, installs your public key |
| Sudoers | `reload`/`restart` of the web server and PHP-FPM, plus the rules for the environment's `services`, validated with `visudo -c`. One file per project |
| Project directory | `remote_path` owned by the deploy user, group `www-data`, setgid. Releases are extracted with `umask 002`, and a new PHP-FPM gets `UMask=0002` (systemd drop-in), so the deploy user (hooks, cron) and PHP-FPM can write each other's files on every distro |
| DB backups | Daily systemd timer dumping the database to `remote_path/backups` (§8) |
| Cron jobs | `provision.cron` in `/etc/cron.d/versa-<project>`: each job runs as the deploy user in `current/app` (e.g. versaWYS `app:schedule:run`). Rewritten whole on every run |
| UFW firewall | Allows SSH (the port in use), 80 and 443 |

### Distro support

| Service | Debian / Ubuntu (apt) | RHEL / Rocky / Alma 8+ (dnf) | e2e |
| :------ | :-------------------- | :--------------------------- | :-- |
| Base packages, `provision.packages` | ✅ | ✅ (+ EPEL) | all |
| PHP-FPM + OPCache | ✅ ondrej (Ubuntu) / sury (Debian) | ✅ AppStream, or Remi for other versions and phpredis | Ubuntu, Debian 8.4 · Rocky 8.3 · Alma 8.4 |
| FrankenPHP | ✅ | ✅ (nginx; check SELinux on enforcing servers) | Ubuntu (versaTodo + Laravel) |
| Nginx | ✅ | ✅ | Ubuntu, Rocky, Alma |
| Apache2 | ✅ | ❌ refused: use nginx | Debian |
| SSL — Let's Encrypt | ✅ | ✅ (certbot from EPEL) | — (needs public DNS) |
| SSL — self-signed | ✅ | ✅ | manual (AlmaLinux) |
| PostgreSQL | ✅ | ✅ | Ubuntu, Rocky |
| MariaDB / MySQL | ✅ | ✅ | Debian, Alma |
| Redis | ✅ | ✅ | Ubuntu, Rocky |
| Dragonfly | ✅ x86_64 + arm64 (.deb) | ✅ x86_64 (.rpm) | Debian, Alma |
| Python 3 + venv | ✅ system, or deadsnakes (Ubuntu) | ✅ system, or AppStream `python3.X` | Ubuntu, Rocky, Alma |
| Node.js | ✅ NodeSource | ✅ AppStream module | Ubuntu, Rocky, Alma |
| Deploy user, sudoers, project dir, backups, cron jobs | ✅ | ✅ (+ SELinux labels) | all |
| UFW | ✅ | ❌ refused: firewalld (nginx opens 80/443) | — |
| Upgrades `U` / `S` / `ctrl+r` | ✅ | ❌ `dnf upgrade` by hand | Ubuntu, Debian |

Deploys (builds, releases, hooks, `services`, rollback) are the same on both: they only need SSH and
POSIX tools on the server.

### 5.1 RHEL family (Rocky/Alma)

Same steps over `dnf`, with these differences:

- **Web server: nginx only.** Apache is provisioned on Debian/Ubuntu only (the vhost goes to `/etc/nginx/conf.d/<project>.conf`). UFW is refused: the nginx step opens `http`/`https` in firewalld when it's active.
- **PHP and Node.js come from AppStream module streams** (`php:8.3`, `nodejs:22`): the step checks the repos offer the requested version before installing, and never switches the stream of an installed one. When AppStream lacks the PHP version (EL9 stops at 8.3) or the app needs Redis/Dragonfly (phpredis), **Remi's repository** is added (`php:remi-8.4`, like ondrej/sury on Debian). `php-process` (posix/pcntl) and `php-sodium` are installed too: on RHEL they don't ship in `php-common`.
- PHP-FPM listens on `/run/php/php<v>-fpm.sock` and answers to `php<v>-fpm` like on Debian (tmpfiles.d keeps `/run/php` across reboots). The pool keeps running as `apache:apache` (its sessions dir is group `apache`) and `apache` joins `www-data`.
- **EPEL** is enabled for nginx's headers-more module, certbot and htop (Rocky/Alma ship it; on RHEL enable it by hand).
- **SELinux**: `remote_path` is labeled `httpd_sys_rw_content_t`, `/run/php` `httpd_var_run_t`, `~/.ssh` restored, and `httpd_can_network_connect` is set for apps behind a proxy. The e2e runs in Docker without SELinux, so check `ausearch -m avc` on a first enforcing server.
- Upgrades: `dnf upgrade` by hand (the view's `U`/`S` are apt only).

### Versions

Version parameters cycle with `Enter` through the **supported** releases, newest first, always at least three:

- **PHP** — e.g. `8.5 8.4 8.3 8.2`
- **Node.js** — LTS majors only, e.g. `26 24 22`
- **Python** — `system` plus e.g. `3.15 3.14 3.13 3.12 3.11` (other than the system one: Ubuntu via deadsnakes, RHEL family when AppStream packages it, e.g. `python3.12`)

The list comes from [endoflife.date](https://endoflife.date) (once per session, 4 s timeout) and falls back to a built-in list offline. When the app uses another Python version, set `builds.python.python_command: python3.13` too.

---

## 6. Safety rules on servers already in use

The scripts are written to be re-run on production servers:

- **Never an alternative**: Apache installed → nginx is refused (and the opposite); Redis installed → Dragonfly is refused (and the opposite: both want port 6379); MySQL installed → it is reused, MariaDB is never installed over it; a PHP-FPM or Node.js version installed → another one is refused; one PHP per server: PHP-FPM installed → FrankenPHP is refused (and the opposite). The TUI blocks the selection and the server script aborts before `apt-get`.
- **No surprise upgrades**: packages that are already installed are skipped, never upgraded. Upgrades only happen with `U` / `S`.
- **Existing services keep their config and keep running**: no changes to `php.ini`, OPCache, `postgresql.conf`, MySQL/MariaDB or Redis config, no restarts. Only what's missing is added (a PHP extension, an Apache module), followed by a graceful reload when needed.
- **Existing database users keep their password** — other apps may use them. The database and privileges are ensured. Changing the password is an explicit action: **`X`** (then update every `.env` that uses it).
- **One vhost per domain**: if another vhost already serves the domain, the install stops and names it. Vhosts are backed up before being replaced and restored when the new one fails `nginx -t` / `apachectl configtest`. The stock default site is only removed on a web server installed in the same run.
- **Firewall**: an active UFW keeps its policy and rules; only SSH, 80 and 443 are allowed.
- **`current/` is never created by hand**: it stays a symlink created by the first deploy.
- **Secrets**: the DB password travels in a file inside a private (0700) temp dir that is removed after the run — never on a command line. `w` never writes `db_password` to `deploy.yml`: keep it as `${DB_PASS}`.

---

## 7. Several apps on one server

Each project gets its own vhost (`sites-available/<project>`), sudoers file (`/etc/sudoers.d/<user>-<project>`), `services` and backup timer. Create one environment (or config file) per app with its own `project`, `remote_path` and `provision.domain`, and install from each one. *Sites on this server* lists them all.

Extra domains go in `provision.aliases` (e.g. `www.myapp.com`): they are added to the vhost, and to the certificate when they resolve to the server.

---

## 8. Backups

The **DB backups** step installs `/usr/local/sbin/versa-backup-<project>` and a daily systemd timer (`versa-backup-<project>.timer`, randomized up to 30 min, catches up after downtime). Each run writes `remote_path/backups/<db>-<date>.sql.gz` (`pg_dump` / `mysqldump --single-transaction`) and keeps the newest `backup_keep` dumps (default 7).

- `B` runs a backup now and prints its size.
- The folder belongs to the deploy user (`0700`; the remote path is setgid `www-data`, so the dumps are kept private to it): download dumps from the **Files** view.
- Restoring is manual on purpose (it overwrites the database), e.g. `gunzip -c dump.sql.gz | sudo -u postgres psql mydb`.

---

## 9. `provision` reference

| Field | Default | Description |
| :---- | :------ | :---------- |
| `ssh.user` / `ssh.key_path` / `ssh.port` | from `ssh` | Admin (root) connection for installs and upgrades |
| `domain` | - | Main domain of the vhost and the certificate |
| `aliases` | `[]` | Extra domains (vhost + certificate) |
| `web_server` | detected, else `nginx` | `nginx` or `apache` |
| `php_version` | installed one, else newest supported | PHP-FPM version |
| `frankenphp` | `false` | Run the PHP app on FrankenPHP (a service on `proxy_port`, default 8000) instead of PHP-FPM; see [examples/deploy.laravel-frankenphp.yml](../examples/deploy.laravel-frankenphp.yml) |
| `python_version` | system | Python installed next to the system one |
| `node_major` | `engines.node`, else newest LTS | Node.js major |
| `doc_root` | `current/app` | Document root relative to `remote_path` (releases keep the code in `app/`; e.g. `current/app/public` for Laravel) |
| `database` | detected | `postgres`, `mariadb` or `none` |
| `db_name` / `db_user` | from `project` | Database and user names (letters, digits, `_`) |
| `db_password` | - | Password for a **new** user (use `${VAR}`; never written by the TUI) |
| `redis` / `dragonfly` / `ssl` / `ufw` / `backup` | `false` | Preselect these services (`redis` and `dragonfly` exclude each other) |
| `backup_keep` | `7` | Dumps kept |
| `ssl_self_signed` | `false` | SSL step makes a self-signed certificate instead of asking Let's Encrypt |
| `packages` | `[]` | Extra system packages the app needs (OCR, image tools…), installed by the base step, never upgraded |
| `cron` | `[]` | `"<min> <hour> <day> <month> <weekday> <command>"` or `"@daily <command>"`, run as `deploy_user` in `current/app`; write `%` as `\%` |
| `deploy_user` | `ssh.user` | User that owns the releases |
| `backends` | detected | Written by the TUI for reference |
| `proxy_port` | builds / per backend | Port of a Python/Go/Node app behind the web server (passed to its service as `PORT`), or FrankenPHP's |

---

## 10. Testing

`go test ./...` covers detection parsing, templates, bundles (`bash -n`), conflicts and the TUI view. The end-to-end test needs Docker able to run systemd containers:

```bash
go test -tags e2e -v -timeout 60m ./test/e2e/
```

It provisions four servers in parallel: **Ubuntu 24.04** (nginx + PostgreSQL + Redis), **Rocky Linux 9** (nginx + PostgreSQL + Redis over dnf, PHP 8.3 from Remi) and **AlmaLinux 9** (nginx + MariaDB + Dragonfly, PHP 8.4 from Remi), each hosting four sample apps — PHP, Python, Go and Node, each with its own domain — and **Debian 12** (Apache + MariaDB + Dragonfly) with versaTodo (`../versaTodo`, skipped when missing), deploys them and checks HTTP and PHP sessions, aliases, backups, cron jobs and extra packages, phpredis against the cache server (and that it only listens on 127.0.0.1), group-writable releases and PHP-FPM's umask, logs, the sites listing, that re-running keeps passwords/configs/processes untouched, and that an alternative web server is refused. `VERSA_E2E_KEEP=1` keeps the containers; `VERSA_E2E_SERVERS=rocky` runs only those servers.
