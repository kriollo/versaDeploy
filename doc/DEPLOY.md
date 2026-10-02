# ⚙️ Configuration Reference (`deploy.yml`)

Every option of the versaDeploy configuration file, with defaults and behavior. For a commented example see [deploy.example.yml](../deploy.example.yml); to generate a starter file run `versa init`. Check a file anytime with `versa config validate [env]`.

- [File structure](#file-structure)
- [Environment variables in the config](#environment-variables-in-the-config)
- [SSH connection (`ssh`)](#ssh-connection-ssh)
- [Environment settings](#environment-settings)
- [Builds (`builds`)](#builds-builds): [PHP](#php-php) · [Go](#go-go) · [Frontend](#frontend-frontend) · [Python](#python-python)
- [Files between releases: shared, preserved, reusable](#files-between-releases-shared-preserved-reusable)
- [Hooks](#hooks)
- [Service reload (`services_reload`)](#service-reload-services_reload)
- [Services (`services`)](#services-services): run a Go/Python process at boot and restart it on deploy
- [Health check (`health_check`)](#health-check-health_check)
- [Notifications (`notifications`)](#notifications-notifications)
- [Performance and limits](#performance-and-limits)
- [Local mode (no SSH)](#local-mode-no-ssh)
- [Server directory layout](#server-directory-layout)
- [Deprecated options](#deprecated-options)

---

## File structure

```yaml
project: "my-app"           # required

environments:
  production:               # the name you pass to `versa deploy production`
    ssh: { ... }
    remote_path: "/var/www/my-app"
    builds: { ... }         # at least one build must be enabled
    shared_paths: [ ... ]
    post_deploy: [ ... ]
    # ...
  staging:
    # ...
```

| Field          | Type   | Description                                                      |
| :------------- | :----- | :--------------------------------------------------------------- |
| `project`      | string | **Required.** Project name, used in logs and notifications.      |
| `environments` | map    | **Required.** At least one environment. Each key is an environment name. |

Every setting below lives **inside an environment** and must be indented one level deeper than the environment name. Settings are not inherited between environments.

You can have several config files in one repo (e.g. one per server: `deploy_server1.yml`, `deploy_server2.yml`); see [Choosing the configuration file](CLI_REFERENCE.md#choosing-the-configuration-file).

## Environment variables in the config

Before parsing, `${NAME}` and `$NAME` are replaced with the value of the environment variable on **your** machine (empty if undefined). Use it to keep secrets out of the file:

```yaml
notifications:
  webhook_url: "${SLACK_WEBHOOK}"
```

This applies to the **whole file, hooks included**. To pass a variable to the server's shell instead, write `$$`, which becomes a literal `$`:

```yaml
post_deploy:
  - "echo $$HOME"                  # the server runs: echo $HOME
  - "cp .env.example $${APP_DIR}"  # the server runs: cp .env.example ${APP_DIR}
  - "awk '{print $1}' file"        # unchanged: $1, $@, $?, $# … are never replaced
```

Only names that start with a letter or `_` are replaced, so positional and special shell variables (`$1`, `$@`, `$?`, `$#`) pass through untouched.

---

## SSH connection (`ssh`)

Required unless the environment uses [local mode](#local-mode-no-ssh).

| Field               | Type   | Default              | Description                                                                                                       |
| :------------------ | :----- | :------------------- | :---------------------------------------------------------------------------------------------------------------- |
| `host`              | string | —                    | **Required.** Server hostname or IP.                                                                              |
| `user`              | string | —                    | **Required.** SSH user. Needs write access to `remote_path`.                                                      |
| `key_path`          | string | —                    | **Required.** Private key. `~/` is expanded. On Linux/macOS the key must be `chmod 600`.                          |
| `port`              | int    | `22`                 | SSH port.                                                                                                         |
| `use_ssh_agent`     | bool   | `false`              | Also try the keys loaded in your SSH agent.                                                                       |
| `known_hosts_file`  | string | `~/.ssh/known_hosts` | File used to verify the server's host key. `~/` is expanded.                                                      |
| `strict_host_key`   | bool   | `false`              | Refuse to connect if the host key can't be verified (missing/unreadable `known_hosts`). See below.                |
| `legacy_algorithms` | bool   | `false`              | Enable SHA1 key exchanges, CBC ciphers and `ssh-dss` host keys for very old servers (OpenSSH < 7, e.g. CentOS 5). Insecure: only when needed. |

Paths can be Unix-style (`~/.ssh/id_rsa`) or Windows-style (`C:\Users\me\.ssh\id_rsa`).

### Host key verification

versa checks the server's identity against `known_hosts`, like `ssh` does:

| Situation                                        | Result                                                                               |
| :----------------------------------------------- | :----------------------------------------------------------------------------------- |
| Server is in `known_hosts` and the key matches   | Connects.                                                                            |
| Server is in `known_hosts` with a different key  | **Refuses** (`key mismatch`): possible man-in-the-middle, or the server was reinstalled. |
| `known_hosts` missing/unreadable                 | Connects with a `Host key NOT verified` warning, or refuses if `strict_host_key: true`. |

The easiest way to add a server is to connect once with the regular `ssh` client and accept the key, or:

```bash
ssh-keyscan -p 22 your-server.com >> ~/.ssh/known_hosts
```

---

## Environment settings

| Field                | Type         | Default                                            | Description                                                                                                                 |
| :------------------- | :----------- | :------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------- |
| `remote_path`        | string       | —                                                  | **Required** (SSH mode). Absolute path on the server; releases, `current`, `shared/` and `deploy.lock` live here.           |
| `ignored_paths`      | list         | `.git`, `tests`, `node_modules/.cache`, `vendor/bin` | Repo-relative paths excluded from change detection and **removed from the release after building** (builds still see them). Setting this list replaces the defaults. |
| `shared_paths`       | list         | `[]`                                               | Paths kept in `<remote_path>/shared` and symlinked into every release. See [below](#files-between-releases-shared-preserved-reusable). |
| `preserved_paths`    | list         | `[]`                                               | Paths copied from the previous release over the new one (the server's copy wins).                                            |
| `pre_deploy_local`   | list         | `[]`                                               | [Hooks](#hooks) run on your machine before building.                                                                        |
| `pre_deploy_server`  | list         | `[]`                                               | [Hooks](#hooks) run on the server before activating the release.                                                            |
| `post_deploy`        | list         | `[]`                                               | [Hooks](#hooks) run on the server after activating the release.                                                             |
| `services_reload`    | list         | `[]`                                               | Commands run on the server right after activation, see [below](#service-reload-services_reload).                            |
| `health_check`       | map          | —                                                  | See [Health check](#health-check-health_check).                                                                             |
| `notifications`      | map          | —                                                  | See [Notifications](#notifications-notifications).                                                                          |
| `route_files`        | list         | `[]`                                               | Files whose change sets `route_cache_regenerate: true` in the release's `manifest.json`.                                     |
| `hook_timeout`       | int (s)      | `300`                                              | Limit for each remote hook, and for `versa exec`.                                                                           |
| `deploy_timeout`     | int (s)      | `600`                                              | Limit for the whole deploy, checked between phases.                                                                         |
| `releases_to_keep`   | int          | `5`                                                | Releases kept on the server; older ones are deleted after each deploy. Also the limit of how far back you can roll back.    |
| `upload_workers`     | int          | `4`                                                | Parallel SFTP uploads.                                                                                                      |
| `chunk_size_mb`      | int          | `10`                                               | Size of each uploaded archive chunk.                                                                                        |
| `incremental_upload` | bool         | `false`                                            | Upload only changed files; see [Performance](#performance-and-limits).                                                      |
| `services`           | list         | `[]`                                               | Processes run by the server's init system, see [Services](#services-services).                                               |
| `local`, `local_path`| bool, string | `false`, —                                         | [Local mode](#local-mode-no-ssh).                                                                                           |

---

## Builds (`builds`)

At least one build must be enabled. All enabled builds run concurrently on your machine, inside a copy of the repository (`HEAD`), so the needed tools (`composer`, `go`, `npm`/`pnpm`, `python3` …) must be installed locally. Builders only run when relevant files changed (or with `--force`).

Paths in `root` are relative to the repository root.

### PHP (`php`)

| Field              | Type   | Default                                                                  | Description                                                                 |
| :----------------- | :----- | :----------------------------------------------------------------------- | :-------------------------------------------------------------------------- |
| `enabled`          | bool   | `false`                                                                  | Enable the PHP builder.                                                     |
| `root`             | string | `""`                                                                     | Directory containing `composer.json`.                                       |
| `composer_command` | string | `composer install --no-dev --optimize-autoloader --classmap-authoritative` | Install command.                                                          |
| `reusable_paths`   | list   | `vendor`                                                                 | Reused from the previous release when `composer.json`/`composer.lock` didn't change. `vendor` is always included. |

Dependencies are reinstalled when `composer.json` or `composer.lock` changes.

### Go (`go`)

| Field         | Type   | Default | Description                                                                                         |
| :------------ | :----- | :------ | :-------------------------------------------------------------------------------------------------- |
| `enabled`     | bool   | `false` | Enable the Go builder.                                                                              |
| `root`        | string | `""`    | Directory containing `go.mod`.                                                                      |
| `target_os`   | string | —       | **Required.** e.g. `linux`.                                                                         |
| `target_arch` | string | —       | **Required.** e.g. `amd64`, `arm64`, `386`.                                                         |
| `binary_name` | string | —       | **Required.** Output file name.                                                                     |
| `deploy_path` | string | `bin`   | Directory **relative to the release root** (not to `app/`) for the binary: the binary ends up at `<remote_path>/current/<deploy_path>/<binary_name>`. Use different paths for several services. |
| `build_flags` | string | `""`    | Extra `go build` flags.                                                                             |
| `cgo`         | bool   | `false` | Build with `CGO_ENABLED=1`. By default binaries are static (`CGO_ENABLED=0`).                       |

To run the binary as a service that starts at boot and is restarted on each deploy, add it to [`services`](#services-services).

Before building, versa compares the target with the server (`uname`) and with the minimum kernel your Go version supports, and stops early if the binary couldn't run there. Go is rebuilt only when `.go` files, `go.mod` or `go.sum` change.

### Frontend (`frontend`)

| Field                | Type   | Default                       | Description                                                                                                       |
| :------------------- | :----- | :---------------------------- | :---------------------------------------------------------------------------------------------------------------- |
| `enabled`            | bool   | `false`                       | Enable the frontend builder.                                                                                      |
| `root`               | string | `""`                          | Directory containing `package.json`.                                                                              |
| `npm_command`        | string | `npm ci --only=production`    | Install command. Runs when `package.json` or its lockfile (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`) changed, or when `node_modules` is missing. Usually you want dev deps for building: `npm ci`, `pnpm install`. |
| `compile_command`    | string | —                             | **Required.** Build command, e.g. `npm run build`. If it contains `{file}`, it runs once per changed frontend file with `{file}` replaced. |
| `cleanup_dev_deps`   | bool   | `false`                       | After building, delete `node_modules` and run `production_command`.                                              |
| `production_command` | string | `pnpm install --production`   | Production-only install used by `cleanup_dev_deps`.                                                               |
| `reusable_paths`     | list   | `node_modules`                | Reused from the previous release when `package.json` and its lockfile didn't change. `node_modules` is always included.           |

### Python (`python`)

Python dependencies are installed **on the server**, in a virtualenv inside each release (`<release>/app/<root>/<venv_path>`). A virtualenv built on your machine wouldn't work there (other OS, paths and compiled packages). When the requirements didn't change, the previous release's virtualenv is reused (hardlinked) instead of reinstalling.

The server therefore needs Python 3 with the `venv` module (Debian/Ubuntu: `apt install python3-venv`), plus `poetry` or `pipenv` if you use them. Your machine doesn't need Python, except for `build_binary`.

| Field                    | Type   | Default            | Description                                                                         |
| :----------------------- | :----- | :----------------- | :---------------------------------------------------------------------------------- |
| `enabled`                | bool   | `false`            | Enable the Python builder.                                                          |
| `root`                   | string | `""`               | Directory of the Python project.                                                    |
| `python_command`         | string | `python3`          | Python **on the server** used to create the virtualenv.                            |
| `package_manager`        | string | `pip`              | `pip`, `poetry` or `pipenv` (the last two must be installed on the server).          |
| `requirements_file`      | string | `requirements.txt` | Dependencies are reinstalled when this file, `pyproject.toml`, `poetry.lock`, `Pipfile` or `Pipfile.lock` changes. |
| `extra_requirements`     | list   | `[]`               | Additional requirements files (pip).                                                |
| `venv_path`              | string | `.venv`            | Virtualenv path, relative to `root`.                                                |
| `reusable_paths`         | list   | `[venv_path]`      | Reused from the previous release when requirements didn't change.                   |
| `install_dev_deps`       | bool   | `false`            | Also install dev dependencies (poetry/pipenv).                                     |
| `use_cache`              | bool   | `false`            | Let pip use its cache (`--no-cache-dir` otherwise).                                 |
| `pypi_mirror`            | string | `""`               | Custom package index URL.                                                           |
| `torch_index`            | string | `""`               | PyTorch index, e.g. `https://download.pytorch.org/whl/cpu`.                         |
| `web_server`             | bool   | `false`            | Generate `run_server.sh` (see below).                                               |
| `web_framework`          | string | —                  | `fastapi`/`uvicorn`, `gunicorn`, `django`, `flask`.                                 |
| `entry_point`            | string | —                  | Module file of the app (e.g. `main.py`, `api/main.py`). Required with `build_binary`. |
| `web_host` / `web_port`  | string / int | `0.0.0.0` / `8000` | Bind address.                                                                 |
| `web_workers`            | int    | —                  | Worker processes (uvicorn, gunicorn; Django defaults to 2, gunicorn to 4).           |
| `web_threads`            | int    | —                  | Threads per worker (gunicorn).                                                      |
| `run_command`            | string | —                  | Replaces the generated start command.                                               |
| `build_binary`           | bool   | `false`            | Build a standalone executable with PyInstaller (requires `entry_point` and `binary_name`). **Only when running versa on Linux**: PyInstaller can't cross-compile. |
| `binary_name`            | string | —                  | PyInstaller output name.                                                            |
| `extra_pyinstaller_args` | string | `""`               | Extra PyInstaller flags.                                                            |
| `service_name`           | string | —                  | **Deprecated**: becomes `services: [{name: ...}]` automatically. Use [`services`](#services-services). |
| `stop_command`, `websocket`, `ws_protocol`, `ws_channel_layer`, `source_path`, `deploy_path` | | | Accepted but not used. |

#### `run_server.sh`

With `web_server: true`, versa writes `run_server.sh` in the Python root of every release. It changes to its own directory, uses the release's virtualenv and `exec`s the server, so the init system tracks the right process:

| `web_framework`         | Command                                                                       |
| :---------------------- | :---------------------------------------------------------------------------- |
| `fastapi` / `uvicorn`   | `.venv/bin/python -m uvicorn <entry>:app --host H --port P [--workers N]`      |
| `gunicorn`              | `.venv/bin/python -m gunicorn <entry>:app -w N -b H:P [--threads T]`           |
| `django`                | `.venv/bin/python -m gunicorn <project>.wsgi:application -w N -b H:P` (finds `<project>/wsgi.py`; falls back to `manage.py runserver` with a warning) |
| `flask`                 | `.venv/bin/python -m flask run --host H --port P` (development server: use `run_command` with gunicorn in production) |
| none, with `entry_point`| `.venv/bin/python <entry_point>`                                               |

`<entry>` is `entry_point` as a module (`api/main.py` → `api.main`). `gunicorn`/`uvicorn` must be in your requirements. Migrations and `collectstatic` aren't run by the script: put them in `post_deploy` hooks, e.g. `".venv/bin/python manage.py migrate --noinput"`.

To run it as a service that starts at boot, add it to [`services`](#services-services).

---

## Files between releases: shared, preserved, reusable

Each deploy creates a fresh release directory. Three mechanisms carry things over from one release to the next:

| Option                    | Where the data lives            | On each deploy                                                                                          | Typical use                              |
| :------------------------ | :------------------------------ | :------------------------------------------------------------------------------------------------------ | :--------------------------------------- |
| `shared_paths`            | `<remote_path>/shared/<path>`   | The path in the release is replaced by a symlink to `shared/<path>`. Whatever the repo had there is discarded. | `.env`, `storage`, `public/uploads`, logs |
| `preserved_paths`         | Inside each release             | Copied from the previous release over the new one. The first deploy uses the repo's version.             | Config files edited on the server        |
| `builds.*.reusable_paths` | Inside each release             | Hardlinked (`cp -al`) from the previous release when dependencies didn't change, instead of reinstalling. | `vendor`, `node_modules`, `.venv`        |

All paths are relative to the app directory (the repository root).

> [!IMPORTANT]
> If a shared path doesn't exist yet in `shared/`, versa creates it **as a directory**. For a shared **file** such as `.env`, create it on the server before the first deploy:
>
> ```bash
> mkdir -p /var/www/app/shared && nano /var/www/app/shared/.env
> ```

---

## Hooks

Commands run at fixed points of the deploy. Each entry is a command string, or a `parallel:` block whose commands run at the same time:

```yaml
pre_deploy_local:
  - "npm test"

pre_deploy_server:
  - "php artisan down || true"

post_deploy:
  - "php artisan migrate --force"
  - parallel:
      - "php artisan config:cache"
      - "php artisan route:cache"
  - "php artisan up"
```

| Hook                | Runs on      | Working directory          | When                                               | If it fails                                                         |
| :------------------ | :----------- | :------------------------- | :------------------------------------------------- | :------------------------------------------------------------------ |
| `pre_deploy_local`  | Your machine | Repository root            | Before checking/cloning the repo. Skipped on `--dry-run`. | Deploy aborts. Nothing was touched on the server.             |
| `pre_deploy_server` | Server       | `<new release>/app`        | After upload, before switching `current`           | Warning only; the deploy continues.                                 |
| `post_deploy`       | Server       | `<new release>/app`        | After switching `current` and `services_reload`    | `current` is switched back to the previous release and the deploy fails (on the first deploy there is nothing to go back to). |

- Remote hooks run in the SSH user's login shell, with a limit of `hook_timeout` seconds each.
- Local hooks (`pre_deploy_local`, and `post_deploy` in local mode) run with `sh -c`. **On Windows, `sh` must be in `PATH`** (it comes with Git for Windows: add `C:\Program Files\Git\bin`).
- On the first deploy (`--initial-deploy`), versa asks before running `post_deploy`, so you can set up `.env` first.
- Re-run `post_deploy` without deploying: `versa hooks <env> [indices...]`.
- `$NAME` is [expanded locally](#environment-variables-in-the-config); write `$$NAME` for a variable of the server's shell.

---

## Service reload (`services_reload`)

Commands run on the server right after `current` is switched (before `post_deploy`), and after every rollback (`versa rollback`, the TUI, and the automatic rollbacks after a failed `post_deploy` hook or health check). Each has a 30 s timeout; failures are warnings and never cause a rollback.

```yaml
services_reload:
  - "sudo systemctl reload php8.2-fpm"
  - "sudo systemctl reload nginx"
```

> [!IMPORTANT]
> **PHP-FPM must be reloaded after every deploy.** OPcache and PHP's `realpath_cache` remember where `current` pointed, so without a reload PHP keeps serving the old release for minutes. The SSH user needs passwordless `sudo` for these commands; see [Getting Started](GETTING_STARTED.md#2-prepare-the-server).

Run them on demand with `versa services-reload <env>`.

---

## Services (`services`)

Long-running processes, such as a Go API or a Python web server, that must keep running, start when the server boots, and pick up each new release. For every service versa:

1. Writes its files to `<remote_path>/.versa/`: a wrapper script (`<name>.sh`) plus a systemd unit, an OpenRC script and a SysV init script. All of them point at `<remote_path>/current`, so they stay valid across deploys and rollbacks.
2. Detects the server's init system and installs the matching file: `/etc/systemd/system/<name>.service` (systemd), or `/etc/init.d/<name>` (OpenRC, or SysV init on RHEL/CentOS 5-6 and other servers without systemd). It only copies it when it changed.
3. Enables it at boot (`systemctl enable`, `rc-update add`, `chkconfig --add` or `update-rc.d`).
4. Restarts it after the `post_deploy` hooks, waits `start_wait` seconds and checks that it's still running. If it isn't, the deploy is **rolled back**.
5. Restarts it again after every rollback (`versa rollback`, the TUI, and automatic rollbacks).

```yaml
services:
  - name: "api"                      # Go binary: exec defaults to it
    env_file: ".env"                 # <remote_path>/shared/.env
    environment:
      PORT: "8080"

  - name: "web"                      # Python with web_server: exec defaults to run_server.sh
```

| Field         | Type    | Default                  | Description                                                                                                  |
| :------------ | :------ | :----------------------- | :----------------------------------------------------------------------------------------------------------- |
| `name`        | string  | —                        | **Required.** Name in the init system (`systemctl status <name>`). Letters, digits, `-`, `_`, `.`, `@`.      |
| `exec`        | string  | see below                | Command line. A leading `./` means relative to `<remote_path>/current`: `./bin/go/api --port 8080`.          |
| `working_dir` | string  | `app` (`app/<python.root>` for Python-only projects) | Directory, relative to `<remote_path>/current`, the process starts in.                    |
| `user`        | string  | `ssh.user`               | Unix user the process runs as.                                                                               |
| `env_file`    | string  | —                        | File under `<remote_path>/shared/` with `KEY=value` lines, loaded into the process environment.               |
| `environment` | map     | —                        | Extra environment variables.                                                                                 |
| `start_wait`  | int (s) | `3`                      | Seconds to wait after (re)starting before checking the process is still running.                            |
| `stop_timeout`| int (s) | `30`                     | Seconds a stop waits for the process to exit after SIGTERM before killing it (SIGKILL). Raise it if your app needs longer to shut down cleanly. |

**Default `exec`** (when omitted):

| Builds enabled                         | Runs                                                       |
| :------------------------------------- | :--------------------------------------------------------- |
| `go`                                   | `<remote_path>/current/<go.deploy_path>/<go.binary_name>`   |
| `python` + `build_binary`              | the PyInstaller binary                                      |
| `python` + `web_server`                | `/bin/sh <python root>/run_server.sh`                       |
| `python` + `entry_point`               | `<venv>/bin/python <entry_point>`                           |
| `go` **and** `python`, or neither      | none: set `exec` (one service per process).                 |

Your program should log to stdout/stderr. Logs go to the journal on systemd (`versa service <env> logs`) and to `/var/log/<name>.log` otherwise.

### Permissions (sudo)

Installing and restarting services needs root. Unless `ssh.user` is `root`, versa runs these commands with `sudo -n` (non-interactive), so the SSH user needs passwordless sudo for them. `versa info <env>` checks it, and this prints the exact sudoers line for the server, with its real binary paths:

```bash
versa service production sudoers
# deploy ALL=(root) NOPASSWD: /usr/bin/systemctl start api, /usr/bin/systemctl stop api, /usr/bin/systemctl restart api, ...
```

Save it on the server with `sudo visudo -f /etc/sudoers.d/versa`. The output adapts to the server:

- It always includes `Defaults:<user> !requiretty`: RHEL/CentOS up to 7 ship `Defaults requiretty`, which makes sudo refuse to run over SSH without a terminal (how versa runs commands).
- On servers without `/etc/sudoers.d` (RHEL/CentOS 5's sudo 1.7), it tells you to add the lines at the end of `/etc/sudoers` with `visudo` as root.
- Commands use this server's absolute paths (e.g. `/sbin/chkconfig`), and versa runs them with those same paths: old sudo has no `secure_path`, so `sudo chkconfig` wouldn't match a `/sbin/chkconfig` rule.

> [!WARNING]
> The sudoers line lets the deploy user install a unit / init script **that it wrote itself** (`.versa/` is owned by that user), and units can run as any user. That's equivalent to giving the deploy user root. If that's not acceptable, keep only the `start`/`stop`/`restart` (and `journalctl`) rules, and install the files once as an admin: copy them from `<remote_path>/.versa/` to `/etc/systemd/system/` (or `/etc/init.d/`) and enable them. versa only uses sudo to install when the file changed, so deploys keep working as long as you don't change the service's settings.

### Init systems

| Server                                         | Init                      | Supervision                                                         |
| :--------------------------------------------- | :------------------------ | :------------------------------------------------------------------ |
| Ubuntu 16.04+, Debian 8+, RHEL/CentOS 7+, …    | systemd                   | Restarted automatically if it crashes (`Restart=always`).           |
| Alpine, Gentoo                                 | OpenRC                    | Started at boot; not restarted if it crashes.                       |
| RHEL/CentOS 5-6, old Debian, busybox systems   | SysV init (`/etc/init.d`) | Started at boot (pidfile + `nohup`); not restarted if it crashes.   |

Manage services by hand with [`versa service`](CLI_REFERENCE.md#versa-service).

---

## Health check (`health_check`)

Verifies the new release after `post_deploy` and the service restart. If every attempt fails, `current` is switched back to the previous release, services are reloaded, and the deploy fails.

```yaml
health_check:
  url: "https://myapp.com/health"   # HTTP GET from YOUR machine
  expected_status: 200
  command: "php artisan about"      # and/or a command on the server, in <new release>/app; exit 0 = healthy
  timeout: 10
  retries: 3
  retry_delay: 2
```

| Field             | Default | Description                                                         |
| :---------------- | :------ | :------------------------------------------------------------------ |
| `url`             | —       | URL requested from the machine running versa.                       |
| `expected_status` | `200`   | HTTP status considered healthy.                                     |
| `command`         | —       | Remote command; healthy if it exits with 0. If both are set, both must pass. |
| `timeout`         | `10`    | Seconds per attempt.                                                |
| `retries`         | `3`     | Attempts before giving up.                                          |
| `retry_delay`     | `2`     | Seconds between attempts.                                           |

---

## Notifications (`notifications`)

Sends a JSON `POST` when a deploy finishes. Works with Slack and Microsoft Teams incoming webhooks (`text` field) and Discord (`content` field). Not sent on `--dry-run`.

```yaml
notifications:
  webhook_url: "${DEPLOY_WEBHOOK}"
  on_success: true
  on_failure: true
```

Payload:

```json
{
  "text": "[SUCCESS] my-app → production: success (release 20261001-183807, 42s)",
  "content": "…same as text…",
  "project": "my-app",
  "environment": "production",
  "release": "20261001-183807",
  "commit": "9f2c1e7…",
  "status": "success",
  "error": "",
  "duration_s": 42.1,
  "timestamp": "2026-10-01T18:38:49Z"
}
```

---

## Performance and limits

- **Change detection**: if nothing changed since the server's `deploy.lock`, the deploy stops before building. Builders only run for the languages whose files changed.
- **Upload**: the release is compressed into `chunk_size_mb` chunks and uploaded by `upload_workers` parallel workers, then streamed into `tar` on the server. versa checks free disk space first.
- **`incremental_upload: true`**: the new release starts as a hardlinked copy of the previous one and only changed files are uploaded. Much faster for large projects with small changes.

  > [!WARNING]
  > With hardlinks, both releases share the same files. Hooks must not modify release files **in place** (e.g. `echo >> file`, `sed -i`), or the previous release changes too and rollbacks stop being clean. Writing a new file and renaming it is safe.

- **`deploy_timeout`** stops a deploy that takes too long overall; **`hook_timeout`** limits each hook.

---

## Local mode (no SSH)

For hosting without SSH (shared hosting with FTP or a file manager). versa builds the project and writes a **complete** release to `local_path`, replacing whatever was there. You then upload the contents of `local_path/app` yourself.

```yaml
environments:
  hosting:
    local: true
    local_path: "./local-releases/hosting"
    builds:
      php:
        enabled: true
    post_deploy:
      - "php versaCLI cache:clear"   # runs locally, in local_path/app
```

- `ssh` and `remote_path` are not needed.
- No release history: no rollback, `shared_paths`, `preserved_paths` or `pre_deploy_server` (they are ignored with a warning), and no `services`.
- Every deploy is a full build.
- Don't point `local_path` at a directory your build writes to (e.g. the frontend's `dist`).
- Only `deploy` and `config validate` work; see the [CLI Reference](CLI_REFERENCE.md#which-commands-work-in-local-mode).

---

## Server directory layout

```
<remote_path>/
├── current -> <remote_path>/releases/20261001-183807   # what your web server serves
├── releases/
│   ├── 20260930-170212/
│   └── 20261001-183807/
│       ├── app/            # your repository (built); hooks run here
│       ├── bin/            # Go binaries (go.deploy_path)
│       └── manifest.json   # release metadata
├── shared/                 # shared_paths
├── deploy.lock             # hashes and metadata of the last deploy
├── .versa/                 # generated service files (wrapper, systemd unit, init scripts)
└── .versa.lock/            # exists only while a deploy is running
```

Point your web server at `<remote_path>/current/app/...` (e.g. `/var/www/app/current/app/public`). Release names are UTC timestamps.

---

## Deprecated options

| Option                | Replacement                                                                                         |
| :-------------------- | :-------------------------------------------------------------------------------------------------- |
| `python.service_name` | `services: [{name: ...}]` (migrated automatically, with a warning). The old `.service` file in the app directory is no longer written. |
| `hook_execution_mode` | `before_switch` → move the hooks to `pre_deploy_server` (done automatically, with a warning). `after_switch` → just remove the option. It can't be combined with `pre_deploy_local`/`pre_deploy_server`. |
