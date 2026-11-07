# Known issues

Found deploying real apps end to end. Each entry: what happened, how to reproduce, the suggested fix and its status.

## From the vservDigitalizacion deploy (2026-11-07, v1.12.0)

Setup: Ubuntu 24.04 in Docker (systemd), Apache + PHP 8.4 + PostgreSQL 16 + Redis, versaWYS app,
provisioned from the Services view. Full procedure: `../vservDigitalizacion/documentacion/Despliegue.md`.

### 1. `rollback` can pick a release whose `post_deploy` failed — high

**Fixed (v1.13.0):** `Deployer.rollback` deletes the failed release after every automatic rollback.

- **What happened:** a deploy failed in `post_deploy` and was rolled back automatically. Its release dir
  stayed in `releases/`. A later `versa rollback production` switched `current` to that failed release,
  which never ran `app:deploy:finish`: the site served assets from the wrong `base_url`.
- **Repro:** deploy OK (A) → deploy with a failing `post_deploy` hook (B, auto-rolled back to A) →
  deploy OK (C) → `versa rollback production` → `current` is B, not A.
- **Fix:** delete the release dir after an automatic rollback (`rollbackAfterHook`), or mark it
  (e.g. a `.versa-failed` file) and skip it in `Rollback`, the Releases view and `releases_to_keep`.

### 2. `rollback` without `--to` picks the newest non-current release — medium

**Fixed (v1.13.0):** `deployer.ReleaseBefore`, used by `versa rollback` and the TUI.

- **What happened (from reading the code):** `Rollback` (`internal/deployer/deployer.go`) sorts releases
  newest first and takes the first one that isn't `current`. When `current` isn't the newest (after a
  rollback), a second `rollback` moves **forward** to the newest release instead of further back.
- **Fix:** pick the newest release older than `current` (or the previous one recorded in `deploy.lock`).

### 3. Provisioning can't install the app's extra system packages — medium

**Fixed (v1.13.0):** `provision.packages` (base step).

- **What happened:** the app's OCR needs `tesseract-ocr tesseract-ocr-spa poppler-utils ghostscript`.
  The Services view has no way to add them, so they were installed by hand as root.
- **Fix:** `provision.packages: [...]`, installed by the base step with `apt_install` (idempotent, never
  upgrades). Names should be validated like `debPkgRe`.

### 4. No cron for the app's scheduler — medium

**Fixed (v1.13.0):** `provision.cron` + the Cron jobs step (`/etc/cron.d/versa-<project>`).

- **What happened:** versaWYS apps need `* * * * * … versaCLI app:schedule:run`. Neither provisioning nor
  deploy sets it up; it was added by hand with `crontab` (same for versaTodo).
- **Fix:** a `provision.cron` entry (or Services item) that writes the deploy user's crontab line once
  for `current/app`, idempotently.

### 5. `$name` in hooks silently becomes empty — low

**Fixed (v1.13.0):** loading the config warns about every unset `$name` outside comments.

- **What happened:** the hook `php -r '$f="…"; $c=json_decode(…)'` was sent as `php -r '="…"; =json_decode(…)'`
  and failed with a PHP parse error. `$$` works and is documented (`doc/DEPLOY.md`), but nothing warns.
- **Fix:** `config validate` (and deploy) warn when a `$NAME` in a hook refers to an unset variable,
  suggesting `$$NAME`.

### 6. HTTPS only via Let's Encrypt — low

**Fixed (v1.13.0):** `provision.ssl_self_signed: true`.

- **What happened:** a staging/Docker server without public DNS can't get a certificate, and versaWYS
  refuses `session.user_cookie.secure: false` in `app:check:deploy --production`. A self-signed vhost
  was set up by hand.
- **Fix:** `provision.ssl: self-signed` for non-public environments (cert + SSL vhost from the same
  template; optionally trust it locally so server-side probes verify).

## From the versaStore deploy (2026-11-07, AlmaLinux 9 + Dragonfly)

Setup: AlmaLinux 9.8 in Docker (systemd, dnf), nginx + PHP 8.4 (Remi) + PostgreSQL 13 + Dragonfly v2.1,
self-signed SSL, versaWYS app with phpredis (cache, carts). Procedure: `../versaSTORE/documentacion/Despliegue.md`.

### 7. Releases read-only for the web server on RHEL — high

**Fixed (v1.13.0):** extraction runs with `umask 002`; new PHP-FPMs get `UMask=0002`, cron jobs `umask 002`.

- **What happened:** releases are extracted with the SSH session's umask: 002 on Debian/Ubuntu (pam_umask),
  022 on RHEL. versaWYS couldn't create `versaWYS/config/` on first boot (503 "No se pudo crear la carpeta").
  Files PHP-FPM created (twig cache) weren't writable by the deploy user either, and the other way round.

### 8. PHP sessions failed on RHEL — high

**Fixed (v1.13.0):** the pool keeps `group = apache` and `apache` joins `www-data`.

- **What happened:** the RHEL support set the pool's group to `www-data`, losing group `apache`, which owns
  `/var/lib/php/session` (770): `session_start(): Permission denied`, every page 500. The e2e PHP fixture
  now starts a session.

### 9. PHP 8.4 and phpredis unavailable on EL9 — medium

**Fixed (v1.13.0):** Remi's repository (`php:remi-X.Y`, `php-pecl-redis6`), plus `php-process`/`php-sodium`.

### 10. Low

- **Fixed (v1.13.0).** **The TUI shows the config picker although `--config` was given** (when the working directory has other
  `deploy*.yml`/`versa_deploy*.yml` files). Esc continues with the given file.
- **Fixed (v1.13.0): with `--initial-deploy` and the `post_deploy` hooks skipped, a failing health check is a warning and `deploy.lock` is written.** **The first deploy reports failure when the app answers 503 until installed**: versaWYS's `/up` is 503
  while `install.token` exists, so `--initial-deploy` fails its health check and writes no `deploy.lock`
  (the release is live). Idea: with `--initial-deploy`, a failing health check is a warning.
- **By design.** **Dragonfly is pinned in `scripts/dragonfly.sh`**: `U` doesn't upgrade it; bump the version and sha256
  together (the command is in the script) and reinstall by hand on servers that want the new one.
- **Fixed (v1.13.0): `U`/`S`/`ctrl+r` are listed only on apt servers.** The Services view still shows `ctrl+r apt` on dnf servers (it answers "Upgrades need an apt-based server").

### 11. nginx 429 on SPA pages — high

**Fixed (v1.13.0):** `limit_req` moved from the server block to `location = /index.php` and the proxy `location /`.

- **What happened:** versaStore's storefront answered its API calls with 429 in the browser. One page load is
  ~100 requests in a second (52 ES-module JS/CSS files, ~30 product images, 8 API calls); the server-level
  `limit_req zone=general burst=50` counted the static files too (they're `access_log off`, so the log only
  showed the API 429s). Existing servers: re-run the Nginx step.
