# 🧩 Services: server setup, updates and backups

The **Services** view of the TUI (`versa`, tab *Services*) turns versaDeploy into a small, agentless server panel: it detects what the server runs, installs the stack your app needs, applies updates, warns about security issues, shows logs and schedules database backups. Everything goes over SSH — nothing is installed permanently on the server besides the services you pick.

> [!NOTE]
> Detection works on any Linux server. **Installing, upgrading and backups need a Debian/Ubuntu (apt) server**; on other systems the view is read-only.

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
2. Run `versa` and open **Services** (← / →). The deploy user doesn't exist yet: the view detects through the root connection and shows *"Deploy user … does not exist yet"*.
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
| Base packages | curl, git, unzip, build-essential, ufw… (no system upgrade: use `U`/`S`) |
| PHP-FPM + OPCache | PHP from ondrej (Ubuntu) or sury (Debian), extensions for your DB and Redis, production `php.ini` and OPCache tuned for symlink deploys (`revalidate_path=1`) |
| Apache2 / Nginx | Web server + the project's vhost (from the versaWYS templates): security headers, rate limiting (nginx), caching, sensitive files blocked, `$realpath_root` for PHP |
| SSL | `certbot certonly` for the domain and the aliases that resolve to the server, then the SSL vhost. Skipped with a hint when DNS doesn't point here |
| PostgreSQL | Server + database + user, `pg_hba.conf` rule for 127.0.0.1 |
| MariaDB / MySQL | MariaDB, or the MySQL already on the server; database (utf8mb4) + user |
| Redis | Bound to 127.0.0.1, protected mode, `maxmemory 256mb` |
| Python 3 | System Python, or another version next to it (Ubuntu: deadsnakes) |
| Node.js | NodeSource LTS |
| Deploy user + SSH key | Creates the user, adds it to `www-data`, installs your public key |
| Sudoers | `reload`/`restart` of the web server and PHP-FPM, plus the rules for the environment's `services`, validated with `visudo -c`. One file per project |
| Project directory | `remote_path` owned by the deploy user, group `www-data`, setgid |
| DB backups | Daily systemd timer dumping the database to `remote_path/backups` (§8) |
| UFW firewall | Allows SSH (the port in use), 80 and 443 |

### Versions

Version parameters cycle with `Enter` through the **supported** releases, newest first, always at least three:

- **PHP** — e.g. `8.5 8.4 8.3 8.2`
- **Node.js** — LTS majors only, e.g. `26 24 22`
- **Python** — `system` plus e.g. `3.15 3.14 3.13 3.12 3.11` (other than the system one: Ubuntu only)

The list comes from [endoflife.date](https://endoflife.date) (once per session, 4 s timeout) and falls back to a built-in list offline. When the app uses another Python version, set `builds.python.python_command: python3.13` too.

---

## 6. Safety rules on servers already in use

The scripts are written to be re-run on production servers:

- **Never an alternative**: Apache installed → nginx is refused (and the opposite); MySQL installed → it is reused, MariaDB is never installed over it; a PHP-FPM or Node.js version installed → another one is refused. The TUI blocks the selection and the server script aborts before `apt-get`.
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
| `python_version` | system | Python installed next to the system one |
| `node_major` | `engines.node`, else newest LTS | Node.js major |
| `doc_root` | `current/app` | Document root relative to `remote_path` (releases keep the code in `app/`; e.g. `current/app/public` for Laravel) |
| `database` | detected | `postgres`, `mariadb` or `none` |
| `db_name` / `db_user` | from `project` | Database and user names (letters, digits, `_`) |
| `db_password` | - | Password for a **new** user (use `${VAR}`; never written by the TUI) |
| `redis` / `ssl` / `ufw` / `backup` | `false` | Preselect these services |
| `backup_keep` | `7` | Dumps kept |
| `deploy_user` | `ssh.user` | User that owns the releases |
| `backends` | detected | Written by the TUI for reference |
| `proxy_port` | builds / per backend | Port of a Python/Go/Node app behind the web server (passed to its service as `PORT`) |

---

## 10. Testing

`go test ./...` covers detection parsing, templates, bundles (`bash -n`), conflicts and the TUI view. The end-to-end test needs Docker able to run systemd containers:

```bash
go test -tags e2e -v -timeout 60m ./test/e2e/
```

It provisions **Ubuntu 24.04** (nginx + PostgreSQL) hosting four sample apps — PHP, Python, Go and Node, each with its own domain — and **Debian 12** (Apache + MariaDB) with versaTodo (`../versaTodo`, skipped when missing), deploys them and checks HTTP, aliases, backups, logs, the sites listing, that re-running keeps passwords/configs/processes untouched, and that an alternative web server is refused. `VERSA_E2E_KEEP=1` keeps the containers.
