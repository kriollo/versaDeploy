# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.19.2] - 2026-11-11

### Fixed

- **A package added to `provision.packages` after the install was never installed by default**: the base step counted as installed (curl, git, unzip), so `versa provision` said "Nothing to install". Detection now checks each package: one missing makes the base step pending, and `--dry-run` / the Services view name it (`absent (missing: zbar-tools)`).
- **Two installs at once on one server broke each other** (the TUI and the CLI, or two admins): apt/dnf runs collided. Installs now take a server-wide lock; the second one waits for the first (up to 10 minutes).

## [1.19.1] - 2026-11-10

### Fixed

- **The TUI crashed (nil pointer) on `Y` after `S` or `U` with nothing to upgrade**: the Services view opened the confirm modal before seeing there were no packages, and confirming ran an empty job. Now it only says "No pending upgrades", and a confirm without a job is never run.
- **TUI sidebar from the startup picker** (several deploy files, no `deploy.yml`): `Tab` highlighted nothing and picking an environment left you in Switch Config. Now `Tab` lands on the first environment, picking one (keys or click) opens its Dashboard, and `Enter` leaves the sidebar. A `deploy.yml` that doesn't exist is no longer listed, and a connection still opening when you switch files is dropped instead of landing on the new file's environment of the same name.
- **Self-signed certificates in the Services view** (`ssl_self_signed: true`): the SSL line showed `x509: certificate signed by unknown authority`. It now shows the certificate's expiry as self-signed and how to trust it on your machine; an untrusted certificate without `ssl_self_signed` points to Let's Encrypt.

## [1.19.0] - 2026-11-10

### Added

- **Each project its PHP and Node version on one server.** PHP-FPM 8.1 for one app, 8.4 for another, FrankenPHP for a third, Node 20 and 22: the install adds the project's version next to the others (nothing to turn on), and every deploy points `<remote_path>/.versa/bin` at the project's `php`/`node`/`npm`, which hooks, cron jobs, services, `server_install`, the health check command, `versa exec` and the TUI terminal run first. The server's default `php` doesn't change when a newer PHP is installed. Node majors install from nodejs.org's official builds into `/opt/versa/node-<major>` (the first one is also the server's `node`). RHEL keeps one PHP-FPM per server (FrankenPHP can go next to it). Tested end to end by `TestMultiVersion`.

### Changed

- **The TUI sidebar lists every deploy file** in the directory, each with its environments. `Tab` + `↑`/`↓` (or a click) on an environment of another file loads that file. Switch Config stays for multi-deploy.

### Fixed

- **`←`/`→` reach the Switch Config view** (only a mouse click did); from it, `←`/`→` move on once a config is loaded.
- **A second project on a server got no vhost and no sudoers** from the default selection: nginx being active skipped the step that writes the project's vhost, and another project's sudoers file counted as this one's. Now the web server step is preselected while the project has no vhost, and sudoers is detected by the project's own file.
- **PHP 8.5 with PHP-FPM failed to install**: OPcache is built into PHP 8.5, which has no `php8.5-opcache` package.

## [1.18.1] - 2026-11-10

### Fixed

- **Config discovery finds any `*deploy*.yml`** (and `.yaml`): `deploy.docker.yml`, `deploy.laravel.yml` (the examples' naming) and similar were missing from the TUI's Switch Config view and from the CLI's file prompt.

## [1.18.0] - 2026-11-10

### Added

- **`versa provision <env>`**: installs the server stack from the terminal, with the same steps, checks and parameters as the Services view (`I`). `--only <ids>`, `--prepare` (`P`), `--reset-db-password` (`X`), `--dry-run`, `-y`. Both use one code path (`internal/deployer/provision.go`).

## [1.17.1] - 2026-11-09

### Changed

- **Dependencies up to date**: Go 1.25 (go.mod), `golang.org/x/crypto` 0.54, `pkg/sftp` 1.13.12, `progressbar` 3.19.1, `x/sync` 0.22 and their indirect dependencies.
- **CI on the Node 24 actions**: `actions/checkout` v7, `setup-go` v7, `upload-artifact` v7, `download-artifact` v8 (the release now fails if a downloaded binary's hash doesn't match the uploaded one) and `softprops/action-gh-release` v3, each pinned by the commit of its official tag.

## [1.17.0] - 2026-11-09

### Added

- **Rollbacks in the deploy history**: `versa rollback` and the TUI's rollbacks are recorded (who, when, back to which release), next to the deploys.
- **`VERSA_DEPLOYER`** names who deployed in `versa history` (default `user@host`); in CI, `VERSA_DEPLOYER: ${{ github.actor }}` (the GitHub Actions example sets it).
- `versa service <env> restart` (and deploys) print `<service> is running` when the service stays up; it used to print nothing on success.
- **`examples/deploy.versawys-frankenphp.yml`**: versaWYS on FrankenPHP behind nginx, tested end to end from `composer create-project versawys/versawys` (PostgreSQL, Redis, cron with `app:schedule:run`, `app:deploy:finish`).
- **Ctrl+C frees the deployment lock**: an interrupted deploy used to leave `.versa.lock` behind, so the next one failed with "Deployment lock already held". The message now also says since when the lock is held and the command that frees it.

### Fixed

- **FrankenPHP: `php` subprocesses ran `frankenphp <script>`**: inside FrankenPHP's CLI, `PHP_BINARY` is the `frankenphp` binary, so PHP that runs PHP_BINARY failed (`unknown command "versaCLI" for "frankenphp"`): versaWYS's `app:deploy:finish` and scheduled tasks, Laravel's `schedule:run`. The `/usr/local/bin/php` wrapper now starts FrankenPHP as `php` (`exec -a`), and is rewritten on servers that have the old one.
- **FrankenPHP showed every PHP error and deprecation**, on web pages and in hooks and cron: its static build has no `php.ini`. The FrankenPHP step writes production settings to `/etc/frankenphp/php.d/00-versa.ini` (`display_errors = Off`, `log_errors = On`, no deprecations, the timezone); files added after it in `php.d/` override it.
- **A deploy without PHP dependency changes failed with "release is missing required PHP dependencies"** when composer.json sets `vendor-dir` (versaWYS: `versaWYS/vendor`): `vendor` was reused from the previous release instead of the configured directory.
- **A deploy that only changed the lockfile** (a dependency update) reinstalled the frontend dependencies but didn't compile, and lost the build output. The frontend now compiles when its dependencies change. The examples list their build output in `reusable_paths` (`public/build`, `.next`, `.output`) so deploys without frontend changes keep it, as the docs now say.
- **The TUI's header disappeared** when a long message (an SSH error) didn't fit the status bar: the bar wrapped onto several lines and pushed the whole view up. The status bar is always one line now, the message cut with "…".
- **Nuxt apps weren't detected as a Node backend** in the Services view (Node.js not preselected) because Nuxt has no `start` script: a service that runs `node`/`npm`, or `provision.node_major`, now marks the app as a Node server too.

## [1.16.0] - 2026-11-09

### Added

- **Fail2ban** in the Services view (`provision.fail2ban`), on Debian/Ubuntu and RHEL/Rocky/Alma: bans failed SSH logins and vulnerability scanners (requests for `.env`, `.git/`, phpMyAdmin, `vendor/phpunit`… that answered 404/403/444), with bans that grow for repeat offenders, through UFW, firewalld or nftables. It never counts an app's normal traffic: nginx's 429, API and dashboard 401/403/422, logins, payment webhooks. Localhost, the admin's IP and `provision.fail2ban_ignore` are never banned; a fail2ban configured by hand is left untouched.

### Fixed

- **Config view editor**: it dropped accented letters and `ñ`, ignored pasted text, and its cursor counted bytes, so editing a line with `→` or `ñ` (like the comments of the examples) scrambled it. It now uses the same editor as the Files and Shared views (`Ctrl+K`, `Ctrl+Home`/`Ctrl+End`, paste, any character).

### Tested

Every e2e server installs fail2ban: the whole e2e's traffic (pages, assets, 429 bursts, deploys) never matches the scanner filter, and a real scanner from another container is banned and blocked. A unit test runs the filter against versaSTORE-like traffic (API errors, rate limits, Transbank/Mercado Pago/Flow webhooks, logins) and scanner requests.

## [1.15.0] - 2026-11-09

### Added

- **FrankenPHP** as the PHP runtime instead of PHP-FPM (`provision.frankenphp: true`, Services view): the official static build (PHP 8.5 with the usual extensions, pinned version and sha256), a `php` CLI for hooks and cron, and a per-project config listening on 127.0.0.1. The app runs as a `services` entry that every deploy restarts and health-checks. nginx keeps the versaWYS/Laravel rules and serves the static files; only `index.php` reaches FrankenPHP, and FrankenPHP runs `index.php` for every request (no other `.php` file can be executed). One PHP per server: PHP-FPM and FrankenPHP refuse each other. Example: `examples/deploy.laravel-frankenphp.yml`.
- **Deploy history on the server**: every deploy, failed ones included, is recorded in `<remote_path>/history/` with who ran it, commit, tag, duration, result and its full log (last 100 deploys, logs of the last 20). `versa history <env> [id]`; the TUI shows the last 10 under Operations → `t`.
- **`deploy_tags`**: an environment only deploys commits with a matching tag (`deploy_tags: "v*"` for production), checked per server in multi-deploys.
- **`on_failure` hooks**: run on the server after a failed deploy, in the release that is live (e.g. `php artisan up` after a failed migration).
- **Notifications**: failures carry the last 20 lines of the deploy's log; with `on_failure`, the first success after a failure is sent as `recovered` (even with `on_success: false`); Telegram via `chat_id`. The payload adds `tags` and `log_tail`.
- **`services[].reload`**: a command run instead of a restart on deploys and rollbacks while the service is up (`versa service <env> reload`); a stopped service or a failed reload is restarted. FrankenPHP uses it to free OPcache: its deploys never restart PHP and drop no requests (the new release runs by itself, keyed by real path like PHP-FPM).
- **`install.sh`**: one-line install for Linux and macOS that checks the release's sha256 (`curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh`).
- **`examples/github-actions.yml`**: staging on every push to `main`, production on every `v*` tag.

### Fixed

- **Apps behind the nginx proxy got the Host without its port** (`$host`): on a non-standard port, an app that builds absolute URLs (Laravel's assets) pointed them to the wrong port and the page loaded without styles. The proxy now passes the Host as the client sent it, like PHP-FPM. **Existing servers: re-run the Nginx step.**

### Tested

The e2e now loads each page like a browser: follows redirects, checks the app's own content, and fetches every CSS, JS, font and image it references (200, non-empty, right content type, same host and port). A deploy under steady traffic must not fail a single request.

New e2e server `franken` (Ubuntu 24.04, nginx + FrankenPHP + PostgreSQL + Redis): versaTodo (versaWYS) and a fresh Laravel with migrations, each on its own FrankenPHP service; versaWYS's blocked files stay blocked; PHP-FPM is refused next to FrankenPHP; a failing `post_deploy` runs `on_failure` and is recorded with its log; `deploy_tags` refuses an untagged commit and deploys the tagged one. Three Laravel deploys under steady traffic on FrankenPHP: 226/226 requests answered (a restart or `frankenphp reload` dropped some). All five e2e servers pass (27 apps, every page with its assets).

## [1.14.0] - 2026-11-08

### Added

- **`examples/`**: `deploy.yml` for Laravel, Next.js, Nuxt, Flask (gunicorn), FastAPI (uvicorn) and Gin, with their provisioning, `.env` handling, hooks and health check. Each one is deployed with the real framework by the e2e test on Ubuntu 24.04 and AlmaLinux 9; a unit test loads them and rejects unknown keys.
- **Edit `.env` and `config.json` from the TUI**: the Shared view lists `preserved_paths` (e.g. versaWYS's `config.json`) and the `shared_paths` not created yet; `Enter` on a file opens it, `e` edits it, `Ctrl+S` saves it on the server keeping its mode. `.env.production`/`.env.local` open as text.

### Changed

- **A missing shared path is seeded instead of becoming an empty directory**: from the release (Laravel's `storage/`, a default config file), else from `<path>.example` (`.env.example`, mode 0640), else an empty `.env*` file (0640). A shared `.env` that wasn't created by hand used to become a *directory*. The deploy warns about each path it creates.

### Fixed

- **Two deploys started in the same second collided** (`failed to compress release: … .tar.gz.001: file exists`), even for different projects: the chunks were named after the release timestamp in `/tmp`. They now sit next to each build's private artifact dir. Found deploying six apps at once.
- File viewer/editor: shows the "Saved" message (it was drawn below the screen), refreshes size and date after saving, and the editor starts at the first line. The Shared view shows a file's real size instead of its disk blocks.

### Tested

The six examples were deployed as a user would, one Ubuntu 24.04 container each: stack installed from the Services view (`I`), `versa deploy --initial-deploy`, `.env` edited in the Shared view, `versa service restart` / `deploy --force`, and a redeploy keeping the edited `.env`. Laravel with PostgreSQL sessions, Redis cache, migrations, cron and `storage:link`.

## [1.13.1] - 2026-11-07

### Added

- **`builds.frontend.server_install`** (e.g. `npm ci --omit=dev`): `node_modules` is installed on the server, for its OS and CPU, instead of shipping yours — native modules built on Windows/macOS don't run on Linux. Runs before the switch (a failure stops the deploy) and is reused when the lockfile didn't change, like Python's virtualenv.
- **SSR apps documented and tested**: Next.js 16 and Nuxt 4 deploy end to end on Ubuntu 24.04 and AlmaLinux 9 (e2e), as a `services` entry behind nginx (`doc/DEPLOY.md` → *SSR apps*).

### Fixed

- **nginx 429 on proxied apps' static files**: behind a proxy (Next, Nuxt, Node, Go, Python) the app serves its own chunks, and they went through the rate-limited location. JS, CSS, images and fonts now go to the app without the limit. **Existing servers: re-run the Nginx step.**
- `compile_command`/`npm_command` starting with `VAR=value` (e.g. `NEXT_TELEMETRY_DISABLED=1 npm run build`) were refused as "build tool not found".

## [1.13.0] - 2026-11-07

### Added

- **RHEL family servers (Rocky/Alma/RHEL 8+, dnf)** in the Services view: base packages, PHP-FPM and Node.js from AppStream module streams, nginx (+ EPEL headers-more), PostgreSQL, MariaDB, Redis, Python, certbot, deploy user, sudoers, project dir and backups. nginx only (Apache and UFW are refused, firewalld gets http/https), SELinux labels for the project dir, `/run/php` and `~/.ssh`, `httpd_can_network_connect` for proxied apps. Upgrades (`U`/`S`) stay apt only. The e2e test provisions Rocky 9 with the same four apps as Ubuntu.
- **`provision.packages`**: extra OS packages the app needs (e.g. `tesseract-ocr`), installed by the base step and never upgraded.
- **`provision.cron`** and the **Cron jobs** step: `/etc/cron.d/versa-<project>`, each job run as the deploy user in `current/app` (e.g. versaWYS `app:schedule:run`).
- **`provision.ssl_self_signed`**: the SSL step makes a self-signed certificate (trusted on the server itself) for staging, LAN or Docker hosts without public DNS.
- A warning when the config uses an unset `$name`: it becomes empty; server-side variables need `$$name`.
- **Dragonfly** (`provision.dragonfly: true`), a Redis-compatible alternative: the official release package (pinned, sha256-checked) bound to 127.0.0.1 with 256 MB and LRU-like eviction. Never installed next to Redis (and the opposite). Tested on Rocky 9 (e2e) and AlmaLinux 9 with versaStore.
- RHEL family: **Remi's repository** when AppStream lacks the PHP version (EL9 stops at 8.3) or the app needs phpredis; `php-process` and `php-sodium` installed like Debian's `php-common`.

### Fixed

- **`rollback` could restore a release whose deploy had failed**: a deploy rolled back after `post_deploy`, services or the health check now deletes its release.
- **`rollback` without `--to` could move forward**: after a rollback, another one went to the newest release instead of the one before the active one (CLI and TUI).
- **Releases were read-only for the web server on RHEL**: they're extracted with the SSH session's umask, 002 on Debian/Ubuntu but 022 on RHEL, so versaWYS couldn't create its first-run config there. Extraction now uses `umask 002` everywhere, a new PHP-FPM runs with `UMask=0002` and `provision.cron` jobs with `umask 002`, so the deploy user and PHP-FPM can write each other's cache and logs.
- **nginx answered 429 to SPA pages**: the vhost rate limit (10 req/s, burst 50) covered static files, and one page of an ES-module SPA (versaStore's storefront: ~50 JS modules + product images + 8 API calls) used up the burst before its own API calls. The limit now applies only to requests that reach the app (`index.php` or the proxied app). **Existing servers: re-run the Nginx step** in the Services view to regenerate the vhost (it keeps a backup).
- **`--config` no longer shows the config picker** when the directory has other `deploy*.yml` files.
- **First deploy before the app is set up**: with `--initial-deploy` and the `post_deploy` hooks skipped, a failing health check (versaWYS answers 503 until installed) is a warning, and `deploy.lock` is written.
- Dragonfly on Debian/Ubuntu: its `.deb` starts the service on install, so versa's config (localhost only) is written before the package, never after.
- Services logs (`L`) on RHEL: PHP-FPM, Redis and Dragonfly log files and units; `U`/`S`/`ctrl+r` are listed only on apt servers.
- The SSL step shows as installed with a self-signed certificate; Dragonfly's version shows without color codes.

## [1.12.0] - 2026-11-06

### Added

- **Mouse in the TUI**: click a tab or an environment, and scroll every view with the wheel (before, only Terminal, Services and Operations scrolled). Select text with `Shift` + drag.

### Fixed

- **TUI Terminal view: the prompt was cut off** (since 1.8.0): the output area left no room for the prompt line, so commands couldn't be seen while typing them.
- **TUI Terminal view no longer grabs the keyboard when you pass through it**: ←/→ and the other shortcuts keep working until you press `Enter` to type a command; `Esc` stops typing and a second `Esc` goes back to the Dashboard.

## [1.11.0] - 2026-11-06

### Security

- **Build artifacts in `/tmp` could be tampered with by another local user**: the artifact dir had a predictable name and chunks were world-readable. The dir is now private (`MkdirTemp`, `0700`) and chunks are `0600` and refuse a pre-created file or symlink. Uploaded chunks on the server are `0600` too.
- **Local Python binaries could bundle packages planted by another local user**: `use_cache` passed the shared `--cache-dir /tmp/pip-cache`. pip now uses its private per-user cache (`--no-cache-dir` when `use_cache` is off).
- **Root provisioning followed symlinks in `remote_path`** (*Project directory* step): a deploy user could get any directory chowned to it. The step refuses a path through a symlink.
- **Services-view sudoers step could hand a root service to the deploy user** when `ssh.user` was root. Those services now run as the deploy user; a root one is refused. Root setup also refuses any uid-0 service user (e.g. `toor`).
- **DB password no longer in `mysql`/`psql` arguments** (visible in `ps`): it goes through stdin, with the SQL escaping mode pinned.
- **Service wrappers in `.versa/` (they export `environment`) are `0600`** when the service runs as `ssh.user`. The provisioning bundle is `0600` and is removed even if the SSH session drops.
- **`env_file` is no longer read by systemd as root** for non-root deploys (a symlink in `shared/` could expose root-readable files): the wrapper loads it as the service user. **Existing servers: run `versa service <env> setup | ssh root@SERVER sh` once** (deploys stop with *needs root setup* until then).
- **Server-reported values in root scripts**: PHP version and package/unit names from detection are validated before the upgrade script uses them; control characters are stripped from the report.
- **Apache/nginx vhosts block what versaWYS's `.htaccess` blocks**: the vhost has `AllowOverride None`, so `versaCLI`, `README.md`, `pnpm-lock.yaml`, `tsconfig.json` and other development files were served. Only real files are blocked (router paths with those extensions still reach `index.php`). **Existing servers: re-run the web server step in the Services view.**
- **TUI**: tab completion quotes and escapes remote names (a crafted directory name ran commands); rsync quotes remote paths with rsync < 3.2.4 and uses the configured `known_hosts_file` / `strict_host_key`.
- **Sudoers rules**: OpenRC/SysV rules allow only `start|stop|restart|status`; command paths come from system dirs, never `PATH`.
- **Self-update** only downloads from https GitHub URLs (redirects included), caps sizes, reads the digest of its own file from the `.sha256` and refuses to downgrade. Releases are prereleases only when the tag has a suffix (`/releases/latest` skipped them).
- **Release workflow**: `contents: read` except the release job, actions pinned to commit SHAs, tag format checked, manual releases only from `main`.
- **Stricter config**: environment names, `project` (no quotes, `$`, backslashes), `shared_paths`/`preserved_paths` (inside the app dir, not `.`), `go.binary_name`; `release_dir` read from the server's `deploy.lock` must be a plain name; incremental uploads only remove clean paths inside the release.
- `--log-file` is `0600`; webhooks don't follow redirects; remote command output is capped at 16 MB; tracked symlinks pointing outside the repo ship as links, not as the file they point to; `ssh.legacy_algorithms` logs a warning.

### Added

- TUI: versaDeploy logo — a splash screen while it starts (before the config picker and the first connection), a compact mark in the header and the full logo beside the Dashboard stats.

### Fixed

- **A failed first deploy left the next one without a rollback target**: when `post_deploy` failed on an initial deploy (no `deploy.lock` yet), the next attempt ignored the live release, so `preserved_paths` (e.g. `config.json`) were lost and a failure left the site down. Without `deploy.lock`, the release `current` points to is now used for both.
- **New `shared_paths` directories are group-writable**: the web server (group `www-data` through the setgid project dir) couldn't write logs, cache or uploads there. Existing shared dirs are left as they are: `chmod g+w` them if the app writes there.
- **TUI `w`/`a` and the Config view wrote to `<repo>/deploy.yml`** instead of the file loaded with `--config` (or found as `versa_deploy.yml`).
- **Vhosts installed `0600`** after the provisioning bundle became private, so the detection (run as the deploy user) showed sites without names. They are `0644` again.
- **Base packages include `cron`** and start it: app schedulers (versaWYS `app:schedule:run`) need it, and minimal images don't ship it.
- TUI Terminal view: ←/→ on an empty prompt switch views instead of getting stuck there.
- A failing hook in a `parallel` post_deploy group rolled back once per failing hook, concurrently; now once, after the group ends.
- Cleaning old releases never deletes the live one (`current`), whatever its name sorts as.
- `--dry-run` in local mode no longer replaces `local_path` or runs hooks.

## [1.10.0] - 2026-11-05

### Security

- **The deploy user could become root through `services`**: its sudoers rule allowed `install` of `.versa/<name>.service` into `/etc`, and `.versa/` is the deploy user's, so it could install any unit, running anything as root. Now only root installs init files: once per server with the new **`versa service <env> setup | ssh root@SERVER sh`** (installs and enables the services and writes `/etc/sudoers.d/versa-<project>`), or the Services view's *Sudoers* step. Deploys only enable, start, stop and restart them. When a service's `name`, `user`, `env_file` or `stop_timeout` changes, they stop with *needs root setup*. `exec`, `environment` and `working_dir` changes still deploy normally. Servers set up before this: run `setup` once to replace the old rule (if you saved it in another file, delete it).
- **`journalctl` under sudo has fixed arguments** (`-u <name> -n 200 -f --no-pager`). `-u <name> *` let the deploy user read every unit's logs or vacuum the journal. Users in `systemd-journal`/`adm` read logs without sudo, with any `--lines`.
- **A service can't run as `root`** unless `ssh.user` is root: the deploy user writes the code it runs.

## [1.9.1] - 2026-11-05

### Security

- **DB backups could be turned into root writes**: the daily backup ran as root inside the project directory, so a symlink planted as `backups/` (by the deploy user, or by PHP-FPM through the group-writable project directory) made root chown and write another path. Every file operation of the backup now runs as the deploy user, and the dumps are private to it (`0700`/`0600`).
- **Project directory no longer group-writable** (`2775` → `2755`): `www-data` (PHP-FPM) could rename `current`, `.versa/` or `backups/` in it. Re-run *Project directory* in the Services view to apply it to existing servers.
- **Deploy user's `~/.ssh` set up as that user**: on an existing user, a symlinked `~/.ssh` made root chmod/chown the target.
- **Root connections always verify the host key**: `provision.ssh` sessions carry the DB password, so they refuse a server missing from `known_hosts` even without `ssh.strict_host_key`.
- **Self-update refuses a release without its `.sha256`** (every release publishes one) and a failed download; requests have a timeout.
- **Stricter provisioning values**: `remote_path` (absolute, safe characters, not a system directory like `/` or `/etc`), `doc_root` (inside `remote_path`), `project`, `web_server` and the PHP/Python/Node versions are checked for every step, since they reach sudoers, unit names and vhosts. The public key file must hold one key.

### Fixed

- **Hook timeouts didn't stop the hook on Ubuntu 25.10+**: these servers ship uutils' `timeout`, which kills only the shell and leaves the hook's processes running until they finish. versa now prefers GNU `gnutimeout` when the server has it, which kills the whole process group. (Found when GitHub's `ubuntu-latest` moved to 26.04.)
- TUI terminal and file transfers: the working directory and `ssh.key_path` are quoted (paths with spaces broke them).

## [1.9.0] - 2026-11-05

### Added

- **TUI Services view** — an agentless server panel over SSH ([doc/SERVICES.md](doc/SERVICES.md)):
  - **Detection** in one POSIX round-trip: state and version of nginx, Apache, PHP-FPM, PostgreSQL, MariaDB/MySQL, Redis, Certbot, Python, Node.js, UFW, the app's service, the deploy user, sudoers, the project directory and the backup timer; every enabled site on the server; the domain's SSL expiry (checked from your machine).
  - **Installs** (Debian/Ubuntu) as root over the new `provision.ssh`, with a live log: a non-interactive port of `setup-vps.sh` plus the versaWYS vhost templates (FastCGI for PHP, reverse proxy for Python/Go/Node, SSL via `certbot certonly`, domain `aliases`).
  - **Backend detection** (PHP, Python, Go, Node) from `builds` or the repo files, preselecting what the app needs. Python/Go/Node apps run as `services`: the view proposes the entry (`w` writes it) and installs the sudoers rules deploys need.
  - **Version pickers** with the supported releases, newest first, at least three: PHP, Node.js LTS, Python (next to the system one via deadsnakes on Ubuntu). From endoflife.date, with an offline fallback.
  - **Updates and security**: pending updates per service (`U`), security updates (`S`), `apt-get update` (`Ctrl+R`); a security banner on Services and the Dashboard (security updates, reboot required, unattended-upgrades off, stale package index).
  - **Logs** of any service (`L`), **daily DB backups** with retention and backup-now (`B`), **prepare a fresh server for deploy** (`P`: deploy user with your public key, sudoers, directory), explicit **DB password reset** (`X`).
  - `w` saves the `provision` block (and the proposed `services`) to `deploy.yml` keeping comments, never `db_password`; `a` adds the suggested `services_reload`.
- **`provision`** block in `deploy.yml` ([doc/DEPLOY.md](doc/DEPLOY.md#server-provisioning-provision)).
- **Several apps per server**: per-project vhost, sudoers file and backup timer.
- **End-to-end test** (`go test -tags e2e ./test/e2e/`): Docker Ubuntu 24.04 and Debian 12 with systemd; four sample apps (PHP, Python, Go, Node) on one server plus versaTodo; checks HTTP, aliases, services, backups, logs, re-runs and refused alternatives.

### Safety on servers already in use

- Never installs an alternative to what the server runs (nginx vs Apache, MariaDB over MySQL, a second PHP-FPM or Node.js major): the TUI blocks it and the scripts abort before `apt-get`.
- Installed packages are never upgraded implicitly (only with `U`/`S`); installed services keep their config and aren't restarted (graceful reload only when needed); existing DB users keep their password; an active UFW keeps its policy; one vhost per domain, with backup and restore when the new config fails.

### Fixed

- **The PHP vendor check ignored `config.vendor-dir`**: projects with `"vendor-dir": "versaWYS/vendor"` failed with "release is missing required PHP dependencies".

## [1.8.1] - 2026-10-02

### Fixed

- **A missing `remote_path` was reported as "Deployment lock already held"**: any failure creating `.versa.lock` was taken for a running deploy. versa now says whether the lock is really held, `remote_path` doesn't exist, or the user can't write there, and `--initial-deploy` creates a missing `remote_path`.
- **Files written into missing folders failed** with a bare "failed to create remote file": uploads and generated files now create missing parent folders on the server, and errors name the path and the cause (missing folder vs. permissions). Renames name both paths.
- **`preserved_paths` in folders the new release doesn't have** (e.g. only on the server) failed the deploy: the folder is now created.
- **`--log-file` (and the TUI's log file) in a folder that doesn't exist** failed; the folder is now created, and the TUI shows a warning instead of silently skipping the log. `versa init --config dir/deploy.yml` and TUI downloads create their folder too.

## [1.8.0] - 2026-10-02

### Added

- **Services (`services`)**: run Go binaries and Python servers through the server's init system. versa writes a wrapper plus a systemd unit, an OpenRC script and a SysV init script to `<remote_path>/.versa/`, installs the one matching the server (only when it changed), enables it at boot, restarts it after `post_deploy` on every deploy and checks it stays up (rolling back otherwise). Every rollback restarts services too. Works on systemd, OpenRC (Alpine) and SysV init (RHEL/CentOS 5-6).
- **`versa service <env> [status|start|stop|restart|logs|install|uninstall|sudoers]`**: manage services by hand; `sudoers` prints the exact sudoers line for the server.
- **`versa info` checks sudo** for the environment's services.
- **`services[].stop_timeout`** (default 30 s): how long a stop waits before SIGKILL (systemd `TimeoutStopSec`, OpenRC `retry`, SysV init script).
- **MIT license** (`LICENSE`).
- **Windows CI**: tests also run on `windows-latest`.

### Changed

- **Python dependencies are installed on the server**: each release gets a virtualenv created there (`python3 -m venv` + pip/poetry/pipenv), reused via hardlinks when requirements don't change. Your machine no longer needs Python (except for `build_binary`).
- **`run_server.sh`** uses the release's virtualenv, `exec`s the server, and no longer runs `migrate`/`collectstatic` (move them to `post_deploy` hooks). Django now runs with gunicorn (`<project>.wsgi`) instead of `runserver`. It is written on every build, so `web_*` changes apply without touching Python files.
- **TUI Dashboard scrolls** (↑/↓, PgUp/PgDn) when the server info doesn't fit, instead of pushing the header off screen.
- **Internal Version**: Version bumped to 1.8.0.
- **`python.service_name` is deprecated**: it's migrated to `services` automatically; the `.service` file is no longer written into the app directory.

### Fixed

- **A failed deploy after a manual rollback re-activated the wrong release**: automatic rollbacks (failed `post_deploy` hook, service or health check) went back to the last *deployed* release from `deploy.lock`, not to the one that was live. After `versa rollback` that re-activated the release you had just rolled back from. They now go back to whatever `current` pointed to.
- **Every runtime error printed the command's full help and the error twice**: help is now only shown for wrong arguments.
- **Python dependencies never reached the server**: `pip install` ran on the developer's machine, into its global Python, and no virtualenv was created in the release.
- **`python.build_binary` on Windows/macOS produced a binary the Linux server can't run**: it now fails with a clear message (PyInstaller can't cross-compile).
- **uvicorn was started with `--threads`**, an option it doesn't have (`web_threads` now only applies to gunicorn).
- **`ssh.known_hosts_file: "~/.ssh/known_hosts"` was never read**: `~` was expanded only in `key_path`, so the host key was not verified and a `Host key NOT verified` warning was printed. `~/` is now expanded in `known_hosts_file` too.
- **`$` in hooks was mangled**: env var interpolation used `os.Expand`, which also replaced `$1`, `$@`, `$?` (`awk '{print $1}'` became `awk '{print }'`) and had no escape. Now only `$NAME`/`${NAME}` are replaced, and `$$` is a literal `$` (`echo $$HOME` runs `echo $HOME` on the server).
- **Lockfile-only changes didn't reinstall dependencies**: only `composer.json`/`package.json`/`go.mod`/the requirements file were compared. Now `composer.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `go.sum`, `poetry.lock`, `Pipfile.lock` count too (the first deploy after upgrading reinstalls once when a lockfile exists).
- **Python dependencies were reinstalled on every deploy** when `pyproject.toml` or `Pipfile` existed. They are now hashed like the requirements file.
- **Lockfiles under `ignored_paths` were not hashed**: `composer.lock`, `pnpm-lock.yaml`, `Pipfile` were never matched by the "critical file" check.
- **Rollbacks didn't reload services**: `versa rollback` (without `--to`), TUI rollbacks and the automatic rollback after a failed `post_deploy` hook now run `services_reload`, so PHP-FPM serves the restored release.
- **Builds failed on Windows when a command had quotes**: commands were passed to `cmd.exe /c` as one escaped argument, so inner quotes reached `cmd` as `\"`. Go builds always failed (`go build -o "C:\..."` → `mkdir "C:: invalid syntax`), as did `composer`/`npm`/`pip` commands with quoted arguments. The command line is now handed to `cmd.exe` as typed.
- **git failed on Windows for repository paths with spaces** when `git` was found through `PATH`: it ran via `cmd /c` with the path unquoted. git is now always executed directly.
- **`no common algorithm for host key` on old servers listed in known_hosts**: when known_hosts held an `ssh-rsa` key, versa asked only for `rsa-sha2-*`, which OpenSSH < 7.2 doesn't support. `ssh-rsa` is accepted again, as the SSH library does by default.

### Documentation

- Rewrote the CLI reference (all commands including `diff`, `info`, `services-reload`, `config validate`, `init --local`; config file discovery; TUI shortcuts; local-mode support), the configuration reference (every option and default, hooks, health check, notifications, local mode, server layout), Getting Started (web root is `current/app/...`), Quick Start, Install (real asset names and URLs) and Troubleshooting.
- `deploy.example.yml`: `root` instead of the never-read `project_root`, `ignored_paths` inside the environment instead of a top-level `ignored`, new SSH/upload options.

## [1.7.0] - 2026-10-02

### Added

- **Server info (`versa info <env>`)**: new alias of `ssh-test` that also prints the server's hostname, OS, kernel/arch, CPU model and cores, CPU/RAM/swap usage, disk and inode usage of `remote_path`, init system (systemd/openrc/upstart/sysvinit), libc (glibc/musl), php/node/python/git versions and available tools. Everything is collected in one POSIX round-trip that works on RHEL/CentOS 5, busybox/Alpine and current distros.
- **TUI Dashboard shows the same server info**, plus a "Runtimes & tools" section.
- **Deploy pre-flight**: the server is probed before building; the platform is logged and the deploy fails early when `tar` is missing on the server. The Go target check reuses this probe.
- **`ssh.strict_host_key`**: refuse to connect when the host key can't be verified against known_hosts.

### Changed

- **Much faster uploads**: SFTP writes are pipelined (up to 64 packets in flight) instead of one 32 KB packet per network round-trip. A 28 MB release at 50 ms RTT went from ~20 s to ~5 s end to end.
- **No reassembled archive on the server**: chunks are streamed into `tar` (`cat chunks | tar -xzf -`), cutting the deploy's peak disk usage to chunks + extracted tree.
- **Fewer SSH round-trips**: shared paths, reused dependencies and preserved paths each run as a single remote script, stale files of incremental uploads are removed with batched `rm`, and old releases are deleted with one command.
- **Host key verification**: when known_hosts is missing or unreadable versa still connects, but now logs a visible man-in-the-middle warning.
- **Faster local hashing**: artifact hashing for incremental uploads runs in parallel; change detection uses `WalkDir` and skips FIFOs/sockets/devices.
- **Internal Version**: Version bumped to 1.7.0.

### Fixed

- **Symlink switch on old busybox**: `current` was swapped with `mv -T`, which old busybox lacks. versa now falls back to `rename(2)` via perl/python, then to `rm` + `mv`.
- **Paths with `$`, quotes, spaces or non-ASCII characters** in remote commands: Go's `%q` was used as if it were shell quoting. All remote paths now use POSIX single-quote escaping.
- **`knownhosts: key mismatch` when only some host keys are known** (e.g. only the RSA key in known_hosts while the server prefers ed25519): versa now asks the server only for the key types known_hosts holds.
- **Hooks that time out kept running on old servers**: OpenSSH < 7.9 ignores the kill signal. When the server has `timeout`, hooks are wrapped with `timeout -s KILL`.
- **Dead connections hung until TCP gave up**: keepalive now closes the connection after 8 intervals without a reply.
- **`.github/`, `.gitignore` and other `.git*` entries were silently left out of the artifact**: only `.git` itself is skipped now. Add them to `ignored_paths` if you don't want them on the server.
- **Truncated archives went undetected**: errors from closing the tar/gzip writers were ignored.
- **Shared-path errors were swallowed** (`mkdir`/`rm` failures) and are now reported.
- **Change detection could leak goroutines** after a hashing error.
- **TUI**: the dashboard could show data from the previously selected environment; the Shared view ran one `du` per entry (now a single call).

## [1.6.1] - 2026-10-01

### Fixed

- **Error suggestions were dropped for wrapped errors**: `FormatError` only recognised a `VersaError` at the top level, so any typed error wrapped with `fmt.Errorf("...: %w")` (e.g. every `failed to load config: ...`) lost its code and suggestion in the CLI output. It now unwraps with `errors.As`.
- **Clear message for mis-indented environments**: when an environment's keys (`hook_timeout`, `builds`, `post_deploy`, ...) are indented at the same level as the environment name, YAML reported `cannot unmarshal !!seq into config.Environment`. versa now reports "environment keys are mis-indented" with a suggestion on how to fix the indentation.

### Changed

- **Internal Version**: Version bumped to 1.6.1.

## [1.6.0] - 2026-10-01

### Added

- **`versa diff [environment]`**: Lists the files a deploy would ship (committed HEAD vs the server's `deploy.lock`), grouped by language, plus which dependency manifests changed. `deploy --dry-run` now prints the same list. Dry runs no longer execute `pre_deploy_local` hooks.
- **Incremental upload** (`incremental_upload: true`, opt-in): each release stores a `.versa-files` hash list; the next deploy hardlinks the previous release (`cp -al`), removes stale/changed paths and uploads only the changed files. Falls back to a full upload when not possible. Hooks must not modify release files in place (e.g. `echo >> file`), since unchanged files are shared with the previous release.
- **Per-environment tuning**: `releases_to_keep` (default 5), `upload_workers` (default 4), `chunk_size_mb` (default 10).
- **Legacy SSH servers**: `ssh.legacy_algorithms: true` enables SHA1 key exchanges, CBC ciphers and ssh-rsa/dss host keys for OpenSSH < 7 (e.g. RHEL/CentOS 5). Insecure; off by default.
- **SSH keepalive**: `keepalive@openssh.com` every 30 s so long uploads and hooks survive NAT/firewall idle timeouts.
- **Command health check**: `health_check.command` runs a remote command in the new release's app dir (exit 0 = healthy); can be combined with `health_check.url`.
- **Go target validation**: before building, the deploy reads `uname -srm` from the server and aborts when `go.target_os`/`go.target_arch` can't run there (all Go Linux architectures mapped; 386 on x86_64 and arm on aarch64 accepted), warns about `GOARM` on ARMv5/v6 boards, and aborts when the local Go toolchain is 1.24+ but the server kernel is older than 3.2.
- **`go.cgo`** option (default `false`).
- **Chat-friendly notifications**: webhook payloads now include `text` (Slack/Teams) and `content` (Discord).

### Changed

- **Disk space check** now estimates the deploy's peak usage (chunks + reassembled archive, then archive + extracted tree) instead of artifact size × 1.2, and on failure reports how much deleting the releases cleanup would remove frees.
- **TUI dashboard** gathers all server stats in a single SSH round-trip, using only `/proc` and POSIX tools; CPU usage is measured over 1 s from `/proc/stat` instead of `mpstat`.
- versa's own remote commands run under `/bin/sh -c` regardless of the user's login shell; user hooks, `services_reload`, `exec` and the terminal still use the login shell.
- `Deploy` and multi-server `DeployWithArtifact` now share one upload/activation path; releases are moved into place with an SFTP rename instead of `mv -T`.
- CI checks `gofmt` and uses the Go version from `go.mod`.
- **Internal Version**: Version bumped to 1.6.0.

### Fixed

- **Go builds were linked against the build machine's glibc**: native builds (e.g. linux/amd64 on linux/amd64) used CGO by default and failed on older servers with `GLIBC_2.xx not found`. Now built with `CGO_ENABLED=0` unless `go.cgo: true`.
- **Go builds on Windows**: the build command used inline `GOOS=... go build`, which `cmd.exe` doesn't understand; target variables are now passed through the process environment.
- **"Insufficient disk space" on old servers (RHEL 5)**: `df` wrapped long LVM device names onto two lines, so the free-space column read the `Use%` value (e.g. `50%` → 50 bytes). Now uses `df -Pk` and a strict parser.
- **TUI dashboard on old servers**: disk, RAM (`free -h`), uptime (`uptime -p`), OS (`/etc/os-release`) and CPU (`mpstat` column layout) were wrong or empty on RHEL/CentOS 5–6.
- **Multi-deploy nil panic**: `DeployWithArtifact` passed a nil changeset to runtime validation, which panicked when Python builds were enabled.
- `CleanupOldReleases` never deletes the newest release, even with a keep count below 1.

## [1.5.0rc] - 2026-08-14

### Added

- **Local deploy mode**: New `local`/`local_path` environment config for hosting without SSH access. `versa deploy <env>` builds the project locally and writes a full release to `local_path` (replacing it entirely on every run) instead of uploading over SSH — upload the contents of `local_path/app` to your hosting manually. There is no release history or rollback in this mode by design: `rollback`, `status`, `exec`, `hooks`, and `services-reload` now return a clear error for local environments instead of failing on a missing SSH config.
- **`versa init` — SSH vs local prompt**: `versa init` now asks whether the project deploys to a VPS (SSH) or to local/no-SSH hosting, and generates the matching `deploy.yml` template. Pass `--local` to skip the prompt in scripts/CI.
- **CLI `versa config validate [environment]`**: Validates `deploy.yml` without connecting to any server; prints a summary of the resolved environment (mode, builds, hook counts) when an environment name is given.
- **CLI `versa services-reload [environment]`**: Runs the configured `services_reload` commands without a full deploy (previously only available from the TUI).
- **CLI multi-environment deploy**: `versa deploy env1,env2` deploys to a comma-separated list of environments sequentially in one command.
- **Self-update checksum verification**: Release binaries are now published with a `.sha256` file; `versa self-update` verifies the downloaded binary's SHA256 before replacing the running executable and aborts on a mismatch.

### Changed

- **Version now injected at build time**: `internal/version.Version` is set via `-ldflags -X ...=<tag>` in the release workflow instead of being hardcoded per release.
- **Structured errors in `ssh`/`config`**: SSH connection/auth/timeout errors and config validation errors now return a `verserrors.VersaError` with an actionable suggestion instead of a raw `fmt.Errorf`. The TUI now surfaces these suggestions in its status/log panes instead of the raw error string.
- **Internal Version**: Version bumped to 1.5.0rc.

### Fixed

- **`versa init` — YAML tab indentation**: The generated `go:` build section used tabs instead of spaces for `root`/`deploy_path`, which is invalid YAML indentation. Fixed to use spaces consistently.

## [1.4.1rc] - 2026-04-01

### Added

- **Multi-deploy (TUI)**: Select multiple config files simultaneously in the config selector using `Space` to toggle and `M` to deploy. All selected configs share a single build artifact (clone + compile once, upload + activate to each server in sequence). Reduces total deploy time proportionally to the number of targets.
- **Initial deploy post_deploy confirmation (CLI)**: When `--initial-deploy` is set, the CLI prompts `Run post_deploy hooks? [y/N]` before executing post-deploy hooks. Lets operators verify the server environment (`.env`, config files) before hooks run.
- **Initial deploy post_deploy confirmation (TUI)**: Same confirmation as the CLI, surfaced as an inline modal in the Operations log view. Press `y` to run hooks or `n`/`Esc` to skip.
- **Expanded config file discovery**: `FindConfigFiles` now recognises `*_deploy.yml`, `*_deploy_*.yml`, `*_deploy.yaml`, and `*_deploy_*.yaml` patterns in addition to the existing `deploy*.yml` / `versa_deploy*.yml` variants.

### Fixed

- **TUI — nil panic on startup without config**: Pressing `←`/`→` or any view-switching key while `m.cfg` was `nil` (no config loaded yet) caused `(*Config).GetEnvironment` to dereference a nil pointer. Added a nil guard in `activeEnvCfg()`.
- **TUI — goroutine leak after every deploy**: `startDeploy` dispatched `waitForConfirmRequest` as a concurrent command but never closed `confirmReqCh` when the deploy finished without requesting confirmation (non-initial deploy or no `post_deploy` hooks). The goroutine blocked forever. Fixed with `defer close(confirmReqCh)` in the deploy goroutine.
- **TUI — crash pressing `c` after multi-deploy**: Pressing `c` (Connect) when no config was loaded (`m.cfg == nil`) scheduled `connectEnvCmd(nil, "")` which panicked at `cfg.GetEnvironment`. Now redirects to the config selector instead.
- **TUI — stuck in Operations after multi-deploy**: After a multi-deploy finished and the user pressed `Esc` to close the log, there was no way to return to the config selector (arrow navigation skips `viewConfigSelector`). `Esc` in the Operations done-state now navigates back to `viewConfigSelector` when `m.cfg == nil`.
- **TUI — view switching while deploy is running**: `←`/`→` arrow keys bypassed the Operations log handler and could switch views mid-deploy. View switching is now blocked while `operations.running` is true.

### Changed

- **Internal Version**: Version bumped to 1.4.1rc.

## [1.4.0rc] - 2026-03-26

### Added

- **Fix: Deploy stale — `services_reload`**: New configuration field `services_reload` in each environment. After every deploy and rollback, versaDeploy executes these commands on the remote server (e.g. `sudo systemctl reload php8.2-fpm`) to clear PHP-FPM OPcache and `realpath_cache`. Without this, deployed changes would not take effect until PHP-FPM workers recycled naturally (up to 120 seconds). See `deploy.example.yml` for Apache + PHP-FPM and Nginx examples.
- **Health Check post-deploy**: New `health_check` config block (`url`, `expected_status`, `timeout`, `retries`, `retry_delay`). After every deploy, versaDeploy performs an HTTP GET from the local machine to the configured URL. If the check fails after all retries, automatic rollback to the previous release is triggered.
- **Webhook Notifications**: New `notifications` config block (`webhook_url`, `on_success`, `on_failure`). versaDeploy sends a JSON POST to the configured webhook at the end of each deployment with project, environment, release, commit, status, error, duration, and timestamp fields.
- **Deploy Timeout enforcement**: New `deploy_timeout` config field (default: 600 seconds). The deployment pipeline now checks the deadline at key stages (build, upload, pre-deploy hooks, symlink switch) and aborts with an error if the timeout is exceeded.
- **Rollback to specific version (`--to`)**: New `--to` flag on `versa rollback <env> --to=<version>`. Validates the target release exists on the server, switches the symlink, and runs `services_reload`.
- **CLI `versa exec`**: New command to execute an arbitrary command on the remote server via SSH and print its output. Example: `versa exec production "df -h"`.
- **CLI `versa logs`**: New command to tail remote log files in real-time using `tail -f`. Defaults to Laravel's `storage/logs/laravel.log` inside the active release. Accepts `--lines N` flag. Example: `versa logs production /var/log/apache2/error.log`.
- **CLI `versa hooks`**: New command to re-execute `post_deploy` hooks on the currently active release. Accepts optional zero-based indices to run specific hooks. Example: `versa hooks production 0 2`.
- **TUI — Terminal SSH view (view 6)**: New interactive remote terminal view accessible via `←`/`→` navigation. Features: command history (↑/↓), remote working directory tracking (`cd` updates prompt), Tab completion for remote paths via `ls -1Ap`, mouse wheel scrolling, `Ctrl+L` to clear, `Esc` to exit. Commands run via SSH with output streamed to a scrollable viewport.
- **TUI — Operations: Re-execute post_deploy hooks (`h`)**: New quick action in the Operations view. Press `h` to re-run all configured `post_deploy` hooks against the currently active release.
- **TUI — Operations: Re-execute services_reload (`l`)**: New quick action in the Operations view. Press `l` to re-run all configured `services_reload` commands (e.g. PHP-FPM reload) without a full redeploy.
- **SSH `ExecuteCommandStreaming`**: New method on the SSH client that runs a command and streams stdout/stderr to provided `io.Writer` instances in real-time, with PTY allocation support.

### Changed

- **TUI — Navigation: removed number key shortcuts**: View switching by pressing `1`–`8` has been removed. Navigate between views exclusively using `←`/`→` arrow keys. This fixes a conflict where pressing `h` in the Operations view was incorrectly triggering "go to previous view" instead of "re-execute hooks".
- **TUI — Navigation: removed vim-style keys**: `h`/`l` (left/right) and `j`/`k` (up/down) keybindings removed from the global keymap to prevent conflicts with operation-specific keys.
- **TUI — Tab bar**: View labels no longer show number key hints (e.g. `1:Dashboard` → `Dashboard`).
- **TUI — Status bar**: Help string updated to `←/→:views` instead of `1-7:views`.
- **Deployer pipeline order**: `services_reload` runs after the symlink switch and before `post_deploy` hooks. `health_check` runs after `post_deploy` hooks.
- **Documentation — `doc/GETTING_STARTED.md`**: Added `EnableSendfile Off` / `EnableMMAP Off` Apache directives, sudoers configuration for the deploy user, and `services_reload` marked as REQUIRED for PHP-FPM setups.
- **Documentation — `doc/CLI_REFERENCE.md`**: Added full reference for `versa exec`, `versa logs`, `versa hooks`, and `versa rollback --to`.
- **Internal Version**: Version bumped to 1.4.0rc.

## [1.3.2rc] - 2026-03-25

### Added

- **TUI — Log view scrolling**: Deploy log viewport now supports user-controlled scrolling. Arrow keys (`↑`/`↓`) and `PgUp`/`PgDn` scroll through the log while a deploy is running or after completion. Auto-scroll to bottom resumes when the user scrolls back to the end. Footer shows `↑↓:scroll  PgUp/PgDn:page` hints.
- **TUI — File editor (View 3)**: Text files in the remote file browser can now be edited inline. Press `e` while viewing a text file to open an editable textarea, `Ctrl+S` to save back to the remote server via SFTP, and `Esc` to cancel. Only available for text files with valid UTF-8 content that are not truncated. Original file permissions are preserved after write.
- **TUI — Deploy options: Skip dirty check**: New toggle in the Operations view (`Skip dirty check`) to bypass the uncommitted-changes validation, equivalent to the CLI `--skip-dirty-check` flag.
- **TUI — Deploy options: Debug mode**: New toggle in the Operations view (`Debug mode`) that activates verbose debug logging during the deploy, equivalent to the CLI `--debug` flag.
- **TUI — Deploy options: Log file path**: New text field in the Operations view to specify an optional log file path. When set, deploy log output is written simultaneously to the TUI viewport and to the specified file via `io.MultiWriter`.
- **SSH `WriteRemoteBytes`**: New method on the SSH client to write arbitrary bytes to a remote file path via SFTP, with permission preservation via `Chmod`.

### Changed

- **TUI — Icons overhaul (Material Design Icons + colors)**: All file browser and sidebar icons replaced with Material Design Icons (Nerd Fonts v3, codepoints `\U000Fxxxx`) with language brand colors (Go `#00ADD8`, Python `#3776AB`, JS `#F7DF1E`, TS `#3178C6`, PHP `#777BB4`, Rust `#DEA584`, folder `#7aa2f7`, etc.). Status indicators use MDI glyphs: running `\U000F0765`, success `\U000F012C`, error `\U000F0159`.
- **TUI — Expanded directory icon mappings**: Added icon mappings for `test/tests/__tests__`, `docs/documentation`, `assets/static/public`, `scripts/bin`, `migrations`, `.vscode`, `tmp/temp/cache`, `deploy/deployments`, `api`.
- **Internal Version**: Version bumped to 1.3.2rc.

## [1.3.1rc] - 2026-03-24

### Fixed

- **Config editor scroll behavior**: Improved upward navigation in the Config editor by adding cursor margins so the viewport no longer follows too aggressively near the file end.
- **Self-update restart path**: Fixed restart resolution after `self-update` to avoid attempting to execute a removed `.old` binary path.
- **Config edit mode key isolation**: Global shortcuts are now blocked while editing config files, so keys like `c` are treated as text input instead of triggering commands.

### Changed

- **Internal Version**: Version bumped to 1.3.1rc.

## [1.3.0rc] - 2026-03-24

### Added

- **`--no-gui` flag**: TUI now launches by default when running `versa` with no subcommand. Use `--no-gui` to get the help output instead. `--gui` is kept for backward compatibility.
- **`pre_deploy_local` hooks**: New hook type that runs local shell commands before cloning the repo. Aborts the deployment on failure. Ideal for running tests, linters, or pre-flight checks locally.
- **`pre_deploy_server` hooks**: New hook type that runs remote commands before the symlink switch (non-fatal — warnings only). Ideal for gracefully stopping services before activation.
- **`hook_execution_mode` migration**: Deprecated `hook_execution_mode: before_switch` configurations are now automatically migrated to `pre_deploy_server` at validation time with a printed warning.

### TUI Overhaul

- **Real terminal height**: All views now receive the actual content height from the window size message instead of using the `heightFromWidth()` heuristic (`width / 3`, clamped 10–40). Layouts no longer break on wide or narrow terminals.
- **Esc handling — universal dismiss**:
  - File viewer (browser): `Esc` closes the viewer and returns to the directory listing.
  - Config view (read-only): `Esc` returns to the Dashboard.
  - Operations (running): `Esc` closes the log output even while an operation is in progress.
- **Left/Right navigation unblocked for config read-only**: Arrow key view-cycling now works from the Config view when not in edit mode.
- **Releases view — scroll window**: Added `viewStart` tracking so large release lists scroll correctly rather than overflowing the terminal.
- **Shared view — scroll window**: Same scroll window added; long shared-path lists no longer overflow.
- **Shared view — Nerd Font icons**: Replaced `📄`/`📁` emoji with `iconForEntry()` calls (consistent with the file browser).
- **Icon codepoint replacement**: Replaced all MDI-range Nerd Font codepoints (`\uf7xx`, `\uf9xx`, `\uea6c`, `\ufc1e`, `\ufd42`) — which render as empty boxes in many terminal/font combinations — with widely supported Font Awesome and Devicon codepoints. Key changes: folders use `\uf07b`/`\uf07c`, shell files use `\uf120` (terminal), lock files use `\uf023`, images use `\uf03e`, Docker uses `\ue7b0`, generic files use `\uf15b`.
- **Sidebar — connection indicators for all envs**: Every environment in the sidebar now shows its connection state dot (connected/connecting/error/idle), not just the active one.
- **Operations footer fix**: While an operation is running, the footer now reads `Esc=close   ←/→=switch views   (running…)` instead of the previous misleading message.
- **Operations emoji cleanup**: Replaced `▶`/`◀`/`⚡` section markers with plain-text `▸`/`◂`/`»` to avoid rendering issues.
- **Config footer fix**: Read-only mode footer now shows `Esc=back` and `←/→=switch views`.
- **Removed `heightFromWidth()`**: The width-based height heuristic function is gone; all views use real terminal dimensions.

### Fixed

- **Builder race condition**: Concurrent PHP/Go/Frontend/Python build goroutines previously wrote directly to `b.result` fields while running in parallel, creating a data race. Results are now collected in goroutine-local variables and merged into `b.result` after `errgroup.Wait()`.
- **Changeset channel deadlock**: Hash worker channels were sized to `len(filesToHash)` which could be arbitrarily large. Channel buffer is now capped at 1000 and jobs are sent in a separate goroutine, preventing blocked sends on repositories with many files.
- **Artifact single-chunk compression**: `Compress()` was calling `CompressChunked` with a 100 MB limit, potentially producing multiple chunks for large artifacts. Now uses `math.MaxInt64` to always produce a single `.tar.gz`.
- **Artifact defer-in-loop**: `defer f.Close()` inside a `filepath.Walk` callback was leaking file descriptors until the walk completed. Replaced with an explicit `f.Close()` immediately after `io.Copy`.
- **SSH agent connection leak**: The `net.Conn` to `SSH_AUTH_SOCK` was never stored and therefore never closed. It is now tracked in `Client.agentConn` and closed in `Client.Close()`.
- **`CheckDiskSpace` path injection**: Remote path was interpolated into the shell command without quoting (`%s`). Changed to `%q` to prevent command injection via unusual path characters.
- **Temp lock file collision**: Multiple concurrent deployments to different environments shared the same `deploy.lock` temp file path (`os.TempDir()/deploy.lock`). Now namespaced as `deploy-<envName>.lock`.

### Changed

- **`hook_execution_mode` deprecated**: The field is still parsed for backward compatibility but triggers a migration warning. Use `pre_deploy_local`, `pre_deploy_server`, and `post_deploy` instead.
- **Internal Version**: Version bumped to 1.3.0rc.

## [1.2.0rc] - 2026-03-23

### Added

- **Go Deploy Path Isolation**: Added `builds.go.deploy_path` to control where Go binaries are stored inside each release (default: `bin`). This enables safer multi-service deployments by separating runtime paths.
- **Hook Execution Mode**: Added `hook_execution_mode` with `after_switch` (default) and `before_switch` modes for rollout strategy control.
- **Runtime Pre-Activation Validation**: Deployments now validate critical runtime artifacts (Go binary, Python runtime script/binary, PHP vendor dependencies) before activating `current`.

### Changed

- **Go Rebuild Semantics**: Go binaries are now rebuilt only when Go files or `go.mod` change. A global `--force` no longer triggers Go rebuilds by itself.
- **Go/Python Runtime Reuse**: Enhanced reuse logic to carry over Go binaries and Python runtime assets/dependencies between releases when dependency inputs are unchanged.
- **Python Server Script Output**: Python web-server setup now always produces Linux-compatible `run_server.sh` artifacts for remote servers, regardless of local OS.
- **`versa init` Template**: Updated default generated `deploy.yml` with `hook_execution_mode`, `go.root`, and `go.deploy_path`.

### Improved

- **Configuration Defaults and Validation**: Added stricter validation and sane defaults for Go/Python deployment paths and hook execution mode.
- **Documentation**: Updated `doc/DEPLOY.md`, `doc/GETTING_STARTED.md`, `deploy.example.yml`, and default `deploy.yml` to reflect the new backend deployment model.

### Version

- **Internal Version**: Version bumped to 1.2.0rc.

## [1.1.0rc] - 2026-03-04

### Added

- **Interactive TUI (`--gui`)**: New terminal UI accessible via `versa --gui`. Provides a full-screen interactive dashboard without leaving the terminal.
  - **Sidebar**: Lists all configured environments with live connection indicators (connected / connecting / error / idle). Navigate with `↑/↓` and switch with `Tab`.
  - **View 1 — Dashboard**: Shows the current active release, remote disk usage, and total release count for the selected environment.
  - **View 2 — Releases**: Scrollable table of all releases with the active one marked. Press `r` to rollback to any selected release instantly.
  - **View 3 — File Browser**: SFTP-backed directory navigator rooted at `remote_path`. `Enter` descends into directories, `Backspace` goes up.
  - **View 4 — Shared Paths**: Lists all configured `shared_paths` with their disk usage (`du -sh`).
  - **View 5 — Deploy**: Toggle form for `dry-run`, `force`, and `initial-deploy` flags. Press `d` to launch the deployment and watch logs stream line-by-line in a scrollable viewport.
  - **Lazy SSH connections**: Each environment connects on first access; reconnect at any time with `c`.
  - **Tokyo Night color palette** via Lip Gloss.

### Improved

- **Logger TUI support**: `logger.NewTUILogger(w io.Writer, verbose, debug bool)` constructor added. When set, log output is written to the provided writer (used for deploy log streaming) instead of stdout.
- **SSH `ReadDir`**: Exposed `Client.ReadDir(path string) ([]os.FileInfo, error)` for SFTP directory listing used by the file browser.

### Dependencies

- Added `github.com/charmbracelet/bubbletea` (Elm-architecture TUI framework)
- Added `github.com/charmbracelet/lipgloss` (ANSI styling)
- Added `github.com/charmbracelet/bubbles` (viewport, spinner, key components)

### Changed

- **Internal Version**: Version bumped to 1.1.0rc.

## [1.0.7rc] - 2026-03-04

### Performance

- **Faster Release Sorting**: Replaced O(n²) bubble sort in `SortReleases` with `sort.Slice` (O(n log n)).
- **Logger Lazy Formatting**: `Debug()` now returns early when debug mode is off, eliminating unnecessary `fmt.Sprintf` allocations in production.
- **Faster Change Detection**: Pre-allocated `filesToHash` slice (capacity 512) to reduce GC pressure on large repos. Ignored paths now use a `map[string]struct{}` for O(1) exact-match lookups instead of linear scans.
- **Hash Timeout**: Added a 30-second per-file context timeout to file hashing goroutines, preventing indefinite hangs on stale NFS mounts or broken file permissions.
- **Parallel Repo Copy**: `copyEntireRepo` now uses a `runtime.NumCPU()` worker pool to copy files in parallel, significantly reducing build preparation time on large repos.
- **Parallel Directory Upload**: `UploadDirectory` now creates remote directories sequentially and uploads files with 4 concurrent workers, reducing SFTP transfer time.
- **Atomic Symlink in One Round-Trip**: `CreateSymlink` now executes the full create-rename-verify sequence in a single SSH command, reducing network round-trips from 3 to 1.
- **Larger Upload Buffer**: `uploadFile` now uses `io.CopyBuffer` with a 256 KB buffer to reduce syscall overhead when uploading large artifacts.
- **Reduced Artifact Memory Peak**: `Compress` default chunk size reduced from 1 GB to 100 MB, lowering peak memory usage during compression.

### Improved

- **Shared `CalculateDirSize`**: Extracted duplicate directory-size calculation into a new `internal/fsutil` package, used by both `builder` and `deployer`.
- **Robust Network Error Detection**: `errors.Wrap` now uses `errors.As(*net.OpError)` for network error classification (timeout, connection refused) before falling back to string matching, making it more reliable across library versions.

### Changed

- **Internal Version**: Version bumped to 1.0.7rc.

## [1.0.6rc] - 2026-01-31

### Added

- **Documentation Overhaul**: Centralized documentation strategy by moving guides to the `doc/` directory.
- **New Guides**: Added `doc/GETTING_STARTED.md` and `doc/CLI_REFERENCE.md` for better onboarding and reference.

### Improved

- **Logging Audit**: Full audit of application logs to standardize levels and improve readability. Debug logs are now properly suppressed unless `--debug` is used.
- **SSH Diagnostics**: Added logger support to the SSH client for better troubleshooting of remote operations.
- **Config Documentation**: Expanded comments and examples in `deploy.example.yml`.
- **Builder Stability**: Improved failure reporting in the build phase with more detailed context.

### Changed

- **Internal Version**: Version bumped to 1.0.6rc.

## [1.0.5rc] - 2026-01-31

### Added

- **Version Command**: Added `versa version` command to display the current version of versaDeploy.

### Fixed

- **Self-Update**: Fixed a bug where the self-update command would fail to detect the latest version from GitHub.

## [1.0.4rc] - 2026-01-31

### Fixed

- **Self-Update**: Fixed a bug where the self-update command would fail to detect the latest version from GitHub.

## [1.0.3rc] - 2026-01-31

### Added

- **Skip Dirty Check**: Added `--skip-dirty-check` flag to the `deploy` command. This allows deployments even when the local repository has uncommitted changes, ensuring that only the last committed state is deployed to the server.

## [1.0.2rc] - 2026-01-30

### Added

- **Parallel Chunked Uploads**: Implemented parallel artifact uploading using 10MB chunks. This significantly reduces deployment time for large artifacts.
- **Deployment Locking**: Added a distributed locking mechanism using atomic remote directory creation to prevent concurrent deployments to the same environment.
- **Release Sorting**: Added deterministic release sorting logic to ensure correct order for rollbacks and cleanup operations.
- **Safer Shell Execution**: Improved remote command security by using proper quoting (`%q`) for all user-provided and generated paths.

### Improved

- **SFTP Performance**: Optimized SFTP throughput by increasing the maximum packet size.
- **Diagnostic Logging**: Enhanced error messages with more context when file preservation or hook execution fails.
- **Repository Validation**: Strengthened internal checks for repository integrity before starting a build.

### Fixed

- **Version Consistency**: Synchronized internal versioning with the release candidate tag.
- **Artifact Generator**: Fixed edge cases in manifest validation for complex directory structures.

## [1.0.1 RC] - 2026-01-30

### Added

- **Force Redeploy Flag**: Added `--force` flag to the `deploy` command to trigger full builds (Composer/NPM/Go) even when no file changes are detected.

### Fixed

- **Frontend Cleanup Logic**: Fixed a bug where `node_modules` cleanup was skipped if `package.json` was not modified, ensuring production-only dependencies are always enforced after a build.

### Improved

- **SSH Cleanup Diagnostics**: The `CleanupOldReleases` function now captures and reports the exact output from the server if a release deletion fails.
- **Builder Diagnostics**: Added real-time directory size measurement (before/after cleanup) and improved command logging in the build process for easier troubleshooting.

## [1.0.0 RC] - 2026-01-29

### Added

- **Concurrent File Hashing**: Significantly improved change detection speed (4-8x faster) by parallelizing SHA256 calculations using a worker pool.
- **Parallel Builds**: Added concurrent execution of PHP (Composer), Go, and Frontend (NPM/Yarn) builds, reducing total build time by up to 60%.
- **Compression Progress Bar**: Added real-time visual feedback during the artifact compression phase.
- **Enhanced Progress Visibility**: Separated compression and upload phases in the UI for better status tracking.

### Optimized

- **Performance Overhaul**: Replaced O(n²) bubble sort with O(n log n) standard library sorting for release management.
- **Code Efficiency**: Removed unused functions and streamlined internal build logic.
- **Resource Management**: Optimized I/O operations for large file handling.

### Fixed

- **CLI Help Duplication**: Fixed an issue where available commands were listed twice in the `--help` output.
- **Concurrent Build Isolation**: Ensured independent build processes do not interfere with each other.

## [0.9.0-beta] - 2026-01-29

### Added

- **Self-Update**: Added `versa self-update` command to automatically detect, download, and install the latest version from GitHub. Includes atomic binary replacement and automatic application restart.
- **Improved Versioning**: Added `versa version` command and centralized version management.
- **Standardized Naming**: Release assets are now named `versa_{os}_{arch}` for better predictability and multi-platform compatibility.
- **Preserved Paths**: New feature to lock specific files or directories (e.g., `.env`, `config.php`) to their server-side versions after the initial deployment.
- **Reusable Build Assets**: Implementation of `reusable_paths` to recover large folders (like `vendor`, `node_modules`, `dist`) from the previous release using fast Linux hardlinks.
- **Shared Paths**: Added support for persistent directories across releases using symlinks to a central `shared/` folder.
- **Intelligent Change Detection**: The deployment tracker now monitors critical file extensions (`.vue`, `.ts`, `.php`) even within `ignored_paths`.
- **Automatic Build Dependencies**: Local builder now automatically runs installation commands (e.g., `pnpm install`) if dependencies are missing during compilation.
- **Absolute Symlinks**: Re-implemented `current` symlink and shared path linking using absolute paths for cross-environment robustness.

### Fixed

- **Post-Deploy Hook Permissions**: Added automatic permission normalization (0775 for directories, 0664 for files) to ensure group write access.
- **Hook Context**: Fixed path resolution for post-deploy hooks to always execute within the absolute path of the new release's `app/` directory.
- **Windows Git Path**: Improved Git executable resolution on Windows to handle non-standard installation paths.
- **Artifact Structure**: Standardized the artifact layout to use a consistent `app/` subdirectory.

### Improved

- **Deployment Logs**: Enhanced logging with clearer status messages for rollbacks, hook execution, and file preservation.
- **Automatic Rollback**: Strengthened the automatic rollback mechanism to ensure environment stability on hook failures.
- **Exhaustive Documentation**: Updated `DEPLOY.md` and `deploy.example.yml` with comprehensive configuration guides.
